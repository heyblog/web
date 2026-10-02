package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

func (service *Service) Current(ctx context.Context, request *http.Request) (User, error) {
	access, err := request.Cookie("heyblog_access_token")
	if err != nil {
		return User{}, newAuthError("unauthenticated", 401, "authentication is required")
	}
	claims, err := verifyToken(access.Value, service.config.AccessSecret, "access", time.Now())
	if err != nil {
		return User{}, err
	}
	value, err := service.redis.Get(ctx, "heyblog:auth:session:"+claims.SessionID).Result()
	if errors.Is(err, redis.Nil) {
		return User{}, newAuthError("session_expired", 401, "authentication is required")
	}
	if err != nil {
		return User{}, repositoryError("read session", err)
	}
	var session sessionRecord
	if json.Unmarshal([]byte(value), &session) != nil || session.UserID != claims.Subject || session.AuthVersion != claims.AuthVersion {
		return User{}, newAuthError("session_expired", 401, "authentication is required")
	}
	user, err := service.repo.userByID(ctx, claims.Subject)
	if err != nil || !isActive(user) || user.AuthVersion != claims.AuthVersion {
		return User{}, newAuthError("unauthenticated", 401, "authentication is required")
	}
	return service.toUser(ctx, user)
}

func (service *Service) Refresh(ctx context.Context, request *http.Request) (User, []string, error) {
	cookie, err := request.Cookie("heyblog_refresh_token")
	if err != nil {
		return User{}, nil, newAuthError("unauthenticated", 401, "authentication is required")
	}
	claims, err := verifyToken(cookie.Value, service.config.RefreshSecret, "refresh", time.Now())
	if err != nil {
		return User{}, nil, err
	}
	value, err := service.redis.Get(ctx, "heyblog:auth:session:"+claims.SessionID).Result()
	if errors.Is(err, redis.Nil) {
		return User{}, nil, newAuthError("session_expired", 401, "authentication is required")
	}
	if err != nil {
		return User{}, nil, repositoryError("read session", err)
	}
	var record sessionRecord
	if err := json.Unmarshal([]byte(value), &record); err != nil || record.UserID != claims.Subject || record.AuthVersion != claims.AuthVersion {
		return User{}, nil, newAuthError("session_expired", 401, "authentication is required")
	}
	user, err := service.repo.userByID(ctx, claims.Subject)
	if err != nil || !isActive(user) || user.AuthVersion != claims.AuthVersion {
		return User{}, nil, newAuthError("session_expired", 401, "authentication is required")
	}
	if err := service.redis.Del(ctx, "heyblog:auth:session:"+claims.SessionID).Err(); err != nil {
		return User{}, nil, repositoryError("rotate session", err)
	}
	access, refresh, err := service.issueSession(ctx, user)
	if err != nil {
		return User{}, nil, err
	}
	authUser, err := service.toUser(ctx, user)
	return authUser, []string{access, refresh}, err
}

func (service *Service) Logout(ctx context.Context, request *http.Request) error {
	if cookie, err := request.Cookie("heyblog_refresh_token"); err == nil {
		if claims, verifyErr := verifyToken(cookie.Value, service.config.RefreshSecret, "refresh", time.Now()); verifyErr == nil {
			if err := service.redis.Del(ctx, "heyblog:auth:session:"+claims.SessionID).Err(); err != nil {
				return repositoryError("delete session", err)
			}
		}
	}
	return nil
}

func (service *Service) issueSession(ctx context.Context, user dbUser) (string, string, error) {
	sessionID, err := randomToken(24)
	if err != nil {
		return "", "", err
	}
	now := time.Now()
	record := sessionRecord{UserID: user.ID, AuthVersion: user.AuthVersion, ExpiresAt: now.Add(service.config.RefreshTTL).Unix()}
	encoded, err := json.Marshal(record)
	if err != nil {
		return "", "", err
	}
	if err := service.redis.Set(ctx, "heyblog:auth:session:"+sessionID, encoded, service.config.RefreshTTL).Err(); err != nil {
		return "", "", repositoryError("store session", err)
	}
	access, err := signToken(tokenClaims{Subject: user.ID, SessionID: sessionID, AuthVersion: user.AuthVersion, TokenType: "access", IssuedAt: now.Unix(), ExpiresAt: now.Add(service.config.AccessTTL).Unix()}, service.config.AccessSecret)
	if err != nil {
		return "", "", err
	}
	refresh, err := signToken(tokenClaims{Subject: user.ID, SessionID: sessionID, AuthVersion: user.AuthVersion, TokenType: "refresh", IssuedAt: now.Unix(), ExpiresAt: now.Add(service.config.RefreshTTL).Unix()}, service.config.RefreshSecret)
	return access, refresh, err
}

func (service *Service) toUser(ctx context.Context, record dbUser) (User, error) {
	permissions, err := service.repo.permissions(ctx, record.ID)
	if err != nil {
		return User{}, repositoryError("read user permissions", err)
	}
	_, oauthErr := service.repo.oauthByUser(ctx, record.ID)
	if oauthErr != nil && !isNotFound(oauthErr) {
		return User{}, repositoryError("read GitHub identity", oauthErr)
	}
	return User{ID: record.ID, Email: record.Email, Username: record.Username, DisplayName: record.DisplayName,
		AvatarURL: record.AvatarURL, Role: Role(record.Role), Permissions: permissions, Active: isActive(record),
		EmailVerified: record.VerifiedAt != nil, HasPassword: record.PasswordHash != nil, HasGitHub: oauthErr == nil,
		AuthVersion: record.AuthVersion, CreatedAt: record.CreatedAt.UTC().Format(time.RFC3339), LastLoginAt: formatTime(record.LastLoginAt)}, nil
}

func formatTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}
