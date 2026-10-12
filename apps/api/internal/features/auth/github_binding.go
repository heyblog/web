package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

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
