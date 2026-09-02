package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (service *Service) githubAccessToken(ctx context.Context, code string) (string, error) {
	form := url.Values{"client_id": {service.config.GithubClientID}, "client_secret": {service.config.GithubClientSecret}, "code": {code}, "redirect_uri": {service.githubCallbackURL()}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := service.httpClient.Do(request)
	if err != nil {
		return "", newAuthError("github_exchange_failed", http.StatusBadGateway, "GitHub login is unavailable")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode/100 != 2 {
		return "", newAuthError("github_exchange_failed", 502, "GitHub login is unavailable")
	}
	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil || payload.AccessToken == "" {
		return "", newAuthError("github_exchange_failed", 502, "GitHub login is unavailable")
	}
	return payload.AccessToken, nil
}

func (service *Service) githubIdentity(ctx context.Context, accessToken string) (githubProfile, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return githubProfile{}, "", err
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := service.httpClient.Do(request)
	if err != nil {
		return githubProfile{}, "", newAuthError("github_identity_failed", http.StatusBadGateway, "GitHub login is unavailable")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode/100 != 2 {
		return githubProfile{}, "", newAuthError("github_identity_failed", 502, "GitHub login is unavailable")
	}
	var profile githubProfile
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&profile); err != nil || profile.ID == 0 || profile.Login == "" {
		return githubProfile{}, "", newAuthError("github_identity_failed", 502, "GitHub login is unavailable")
	}
	emailRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return githubProfile{}, "", err
	}
	emailRequest.Header.Set("Authorization", "Bearer "+accessToken)
	emailRequest.Header.Set("Accept", "application/vnd.github+json")
	emailResponse, err := service.httpClient.Do(emailRequest)
	if err != nil {
		return githubProfile{}, "", newAuthError("github_identity_failed", http.StatusBadGateway, "GitHub login is unavailable")
	}
	defer func() { _ = emailResponse.Body.Close() }()
	if emailResponse.StatusCode/100 != 2 {
		return githubProfile{}, "", newAuthError("github_identity_failed", 502, "GitHub login is unavailable")
	}
	var emails []githubEmail
	if err := json.NewDecoder(io.LimitReader(emailResponse.Body, 1<<20)).Decode(&emails); err != nil {
		return githubProfile{}, "", err
	}
	for _, item := range emails {
		if item.Primary && item.Verified {
			return profile, normalizeEmail(item.Email), nil
		}
	}
	return githubProfile{}, "", newAuthError("github_email_required", 409, "a verified GitHub email is required")
}
