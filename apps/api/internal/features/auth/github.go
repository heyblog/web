package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

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

type githubAdapter struct{ service *Service }

func (adapter githubAdapter) authorizationURL(state string) (string, error) {
	s := adapter.service
	if s.config.GithubClientID == "" || s.config.GithubClientSecret == "" {
		return "", newAuthError("github_unavailable", 503, "GitHub login is unavailable")
	}
	return "https://github.com/login/oauth/authorize?" + url.Values{"client_id": {s.config.GithubClientID}, "redirect_uri": {s.oauthCallbackURL(ProviderGitHub)}, "scope": {s.config.GithubScope}, "state": {state}}.Encode(), nil
}

func (adapter githubAdapter) identity(ctx context.Context, code string) (oauthIdentity, error) {
	token, err := adapter.service.githubAccessToken(ctx, code)
	if err != nil {
		return oauthIdentity{}, err
	}
	profile, email, err := adapter.service.githubIdentity(ctx, token)
	if err != nil {
		return oauthIdentity{}, err
	}
	if !validEmail(email) {
		return oauthIdentity{}, newAuthError("github_email_required", 409, "A verified GitHub email is required")
	}
	encoded, err := json.Marshal(profile)
	return oauthIdentity{Subject: fmt.Sprintf("%d", profile.ID), Login: profile.Login, Name: profile.NameOrLogin(), Email: email, Profile: encoded}, err
}

func (profile githubProfile) NameOrLogin() string {
	name := strings.TrimSpace(profile.Name)
	if name != "" {
		return name
	}
	return profile.Login
}
