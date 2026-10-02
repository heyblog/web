package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type githubState struct {
	Intent string `json:"intent"`
	Next   string `json:"next"`
}
type githubProfile struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}
type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func (service *Service) GithubStart(ctx context.Context, next string, bind bool) (string, string, error) {
	if service.config.GithubClientID == "" || service.config.GithubClientSecret == "" {
		return "", "", newAuthError("github_unavailable", 503, "GitHub login is unavailable")
	}
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		next = "/dashboard"
	}
	stateToken, err := randomToken(24)
	if err != nil {
		return "", "", err
	}
	state := githubState{Intent: "login", Next: next}
	if bind {
		state.Intent = "bind"
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		return "", "", err
	}
	if err := service.redis.Set(ctx, "heyblog:auth:github:"+stateToken, encoded, 10*time.Minute).Err(); err != nil {
		return "", "", err
	}
	target := "https://github.com/login/oauth/authorize?" + url.Values{"client_id": {service.config.GithubClientID}, "redirect_uri": {service.githubCallbackURL()}, "scope": {service.config.GithubScope}, "state": {stateToken}}.Encode()
	return target, stateToken, nil
}

func (service *Service) githubCallbackURL() string {
	return strings.TrimRight(service.config.WebBaseURL, "/") + "/auth/github/callback"
}

func (service *Service) GithubCallback(ctx context.Context, request *http.Request, code, stateToken string, stateCookie string) (User, []string, string, error) {
	if code == "" || stateToken == "" || stateCookie == "" || stateToken != stateCookie {
		return User{}, nil, "", newAuthError("invalid_oauth_state", 400, "OAuth state is invalid")
	}
	value, err := service.redis.GetDel(ctx, "heyblog:auth:github:"+stateToken).Result()
	if errors.Is(err, redis.Nil) {
		return User{}, nil, "", newAuthError("invalid_oauth_state", 400, "OAuth state is invalid")
	}
	if err != nil {
		return User{}, nil, "", repositoryError("consume OAuth state", err)
	}
	var state githubState
	if err := json.Unmarshal([]byte(value), &state); err != nil {
		return User{}, nil, "", err
	}
	token, err := service.githubAccessToken(ctx, code)
	if err != nil {
		return User{}, nil, "", err
	}
	profile, email, err := service.githubIdentity(ctx, token)
	if err != nil {
		return User{}, nil, "", err
	}
	if state.Intent == "bind" {
		return service.githubBind(ctx, request, profile, email, state.Next)
	}
	user, err := service.githubLogin(ctx, profile, email)
	if err != nil {
		return User{}, nil, "", err
	}
	record, err := service.repo.userByID(ctx, user.ID)
	if err != nil {
		return User{}, nil, "", err
	}
	access, refresh, err := service.issueSession(ctx, record)
	if err != nil {
		return User{}, nil, "", err
	}
	return user, []string{access, refresh}, state.Next, nil
}

func (service *Service) githubLogin(ctx context.Context, profile githubProfile, email string) (User, error) {
	providerID := fmt.Sprintf("%d", profile.ID)
	oauth, err := service.repo.oauthByProviderID(ctx, providerID)
	var record dbUser
	if err == nil {
		record, err = service.repo.userByID(ctx, oauth.UserID)
		if err == nil && record.VerifiedAt == nil && record.Email != nil && normalizeEmail(*record.Email) == email {
			err = service.repo.verifyEmail(ctx, record.ID)
		}
	} else if !isNotFound(err) {
		return User{}, err
	}
	if isNotFound(err) || record.ID == "" {
		record, err = service.repo.userByEmail(ctx, email)
		if isNotFound(err) {
			username, usernameErr := service.availableGithubUsername(ctx, profile.Login, providerID)
			if usernameErr != nil {
				return User{}, usernameErr
			}
			record, err = service.repo.createUser(ctx, username, email, profile.NameOrLogin(), nil)
			if err == nil {
				err = service.repo.verifyEmail(ctx, record.ID)
			}
		} else if err == nil {
			if err = service.repo.verifyEmail(ctx, record.ID); err != nil {
				return User{}, err
			}
		} else {
			return User{}, err
		}
	}
	if err != nil {
		return User{}, err
	}
	profileJSON, err := json.Marshal(profile)
	if err != nil {
		return User{}, err
	}
	if err := service.repo.upsertOAuth(ctx, record.ID, providerID, profile.Login, profileJSON); err != nil {
		return User{}, err
	}
	if err := service.repo.recordLogin(ctx, record.ID); err != nil {
		return User{}, err
	}
	updated, err := service.repo.userByID(ctx, record.ID)
	if err != nil {
		return User{}, err
	}
	return service.toUser(ctx, updated)
}

func (profile githubProfile) NameOrLogin() string {
	name := strings.TrimSpace(profile.Name)
	if name != "" {
		return name
	}
	return profile.Login
}

func githubUsername(login, providerID string) string {
	normalized := strings.ToLower(login)
	var builder strings.Builder
	for _, char := range normalized {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' {
			builder.WriteRune(char)
		} else {
			builder.WriteByte('_')
		}
	}
	value := strings.Trim(builder.String(), "_")
	if len(value) < 3 {
		value = "user"
	}
	if len(value) > 23 {
		value = value[:23]
	}
	suffix := providerID
	if len(suffix) > 8 {
		suffix = suffix[len(suffix)-8:]
	}
	return value + "_" + suffix
}

func (service *Service) availableGithubUsername(ctx context.Context, login, providerID string) (string, error) {
	base := githubUsername(login, providerID)
	for attempt := 1; attempt <= 100; attempt++ {
		candidate := base
		if attempt > 1 {
			suffix := fmt.Sprintf("_%d", attempt)
			candidate = strings.TrimRight(base[:min(len(base), 32-len(suffix))], "_") + suffix
		}
		if _, err := service.repo.userByUsername(ctx, candidate); isNotFound(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", newAuthError("github_username_unavailable", 409, "a local username could not be allocated")
}

func (service *Service) githubBind(ctx context.Context, request *http.Request, profile githubProfile, email, next string) (User, []string, string, error) {
	current, err := service.Current(ctx, request)
	if err != nil {
		return User{}, nil, next, err
	}
	if current.Email == nil || normalizeEmail(*current.Email) != email {
		return User{}, nil, next, newAuthError("github_bind_email_mismatch", 409, "GitHub primary email must match the current account email")
	}
	providerID := fmt.Sprintf("%d", profile.ID)
	if existing, err := service.repo.oauthByProviderID(ctx, providerID); err == nil && existing.UserID != current.ID {
		return User{}, nil, next, newAuthError("github_account_conflict", 409, "GitHub account is already linked")
	} else if err != nil && !isNotFound(err) {
		return User{}, nil, next, err
	}
	if existing, err := service.repo.oauthByUser(ctx, current.ID); err == nil && existing.ProviderID != providerID {
		return User{}, nil, next, newAuthError("github_already_bound", 409, "the current account already has a GitHub identity")
	} else if err != nil && !isNotFound(err) {
		return User{}, nil, next, err
	}
	profileJSON, err := json.Marshal(profile)
	if err != nil {
		return User{}, nil, next, err
	}
	if err := service.repo.upsertOAuth(ctx, current.ID, providerID, profile.Login, profileJSON); err != nil {
		return User{}, nil, next, err
	}
	updated, err := service.repo.userByID(ctx, current.ID)
	if err != nil {
		return User{}, nil, next, err
	}
	authUser, err := service.toUser(ctx, updated)
	return authUser, nil, next, err
}

func (service *Service) GithubUnbind(ctx context.Context, request *http.Request) (User, []string, error) {
	current, err := service.Current(ctx, request)
	if err != nil {
		return User{}, nil, err
	}
	if !current.HasPassword {
		return User{}, nil, newAuthError("password_required", 409, "set a password before unlinking GitHub")
	}
	if !current.HasGitHub {
		return User{}, nil, newAuthError("github_not_bound", 404, "GitHub is not linked")
	}
	if err := service.repo.unlinkOAuth(ctx, current.ID); err != nil {
		return User{}, nil, err
	}
	updated, err := service.repo.userByID(ctx, current.ID)
	if err != nil {
		return User{}, nil, err
	}
	access, refresh, err := service.issueSession(ctx, updated)
	if err != nil {
		return User{}, nil, err
	}
	authUser, err := service.toUser(ctx, updated)
	return authUser, []string{access, refresh}, err
}
