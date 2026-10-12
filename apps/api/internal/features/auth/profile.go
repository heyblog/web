package auth

import (
	"context"
	"net/http"
	"strings"
	"unicode/utf8"
)

type ProfileConfirmation struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

type DisplayProfile struct {
	DisplayName string `json:"display_name"`
}
type UsernameChange struct {
	Username string `json:"username"`
}

func normalizedProfile(input ProfileConfirmation) (ProfileConfirmation, error) {
	input.Username = strings.ToLower(strings.TrimSpace(input.Username))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if !usernamePattern.MatchString(input.Username) {
		return input, newAuthError("invalid_username", 422, "username must contain 3 to 32 lowercase letters, digits or underscores")
	}
	if input.DisplayName == "" || utf8.RuneCountInString(input.DisplayName) > 128 {
		return input, newAuthError("invalid_display_name", 422, "display name must contain 1 to 128 characters")
	}
	return input, nil
}

func (service *Service) ConfirmProfile(ctx context.Context, request *http.Request, input ProfileConfirmation) (User, []string, error) {
	current, err := service.CurrentSession(ctx, request)
	if err != nil {
		return User{}, nil, err
	}
	input, err = normalizedProfile(input)
	if err != nil {
		return User{}, nil, err
	}
	record, changed, err := service.repo.changeProfile(ctx, profileMutation{UserID: current.ID, AuthVersion: current.AuthVersion, Username: input.Username, DisplayName: input.DisplayName, Confirm: true})
	if err != nil {
		return User{}, nil, err
	}
	return service.profileResult(ctx, record, changed)
}

func (service *Service) ChangeUsername(ctx context.Context, request *http.Request, username string) (User, []string, error) {
	current, err := service.Current(ctx, request)
	if err != nil {
		return User{}, nil, err
	}
	input, err := normalizedProfile(ProfileConfirmation{Username: username, DisplayName: current.DisplayName})
	if err != nil {
		return User{}, nil, err
	}
	record, changed, err := service.repo.changeProfile(ctx, profileMutation{UserID: current.ID, AuthVersion: current.AuthVersion, Username: input.Username})
	if err != nil {
		return User{}, nil, err
	}
	return service.profileResult(ctx, record, changed)
}

func (service *Service) ChangeDisplayName(ctx context.Context, request *http.Request, name string) (User, error) {
	current, err := service.Current(ctx, request)
	if err != nil {
		return User{}, err
	}
	input, err := normalizedProfile(ProfileConfirmation{Username: current.Username, DisplayName: name})
	if err != nil {
		return User{}, err
	}
	record, _, err := service.repo.changeProfile(ctx, profileMutation{UserID: current.ID, AuthVersion: current.AuthVersion, DisplayName: input.DisplayName})
	if err != nil {
		return User{}, err
	}
	return service.toUser(ctx, record)
}

func (service *Service) profileResult(ctx context.Context, record dbUser, renamed bool) (User, []string, error) {
	var cookies []string
	if renamed {
		access, refresh, err := service.issueSession(ctx, record)
		if err != nil {
			return User{}, nil, err
		}
		cookies = []string{access, refresh}
	}
	user, err := service.toUser(ctx, record)
	return user, cookies, err
}
