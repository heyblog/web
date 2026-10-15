package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const blogsClubEndpoint = "https://www.blogsclub.org/Oauth/getOauthData"

type blogsClubAdapter struct{ service *Service }

type blogsClubProfile struct {
	OpenID   string `json:"openid"`
	BlogName string `json:"blog_name"`
	Avatar   string `json:"blogger_avatar"`
	Verified int    `json:"verified"`
	BlogURL  string `json:"blog_url"`
	Email    string `json:"email"`
}

func (adapter blogsClubAdapter) callbackURL() string {
	return strings.TrimRight(adapter.service.config.WebBaseURL, "/") + "/auth/blogsclub/callback"
}

func (adapter blogsClubAdapter) authorizationURL(state string) (string, error) {
	config := adapter.service.config
	if config.BlogsClubClientID == "" || config.BlogsClubClientSecret == "" {
		return "", newAuthError("blogsclub_unavailable", 503, "BlogsClub login is unavailable")
	}
	return "https://www.blogsclub.org/Oauth/Login?" + url.Values{"client_id": {config.BlogsClubClientID}, "redirect_uri": {adapter.callbackURL()}, "response_type": {"code"}, "scope": {"basic,email"}, "state": {state}}.Encode(), nil
}

func (adapter blogsClubAdapter) identity(ctx context.Context, code string) (oauthIdentity, error) {
	config := adapter.service.config
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {adapter.callbackURL()}, "client_id": {config.BlogsClubClientID}, "client_secret": {config.BlogsClubClientSecret}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, blogsClubEndpoint+"?action=token", strings.NewReader(form.Encode()))
	if err != nil {
		return oauthIdentity{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var token struct {
		Status      int    `json:"status"`
		AccessToken string `json:"access_token"`
		OpenID      string `json:"openid"`
	}
	if err := adapter.read(request, &token); err != nil || token.Status != 200 || token.AccessToken == "" || token.OpenID == "" {
		return oauthIdentity{}, newAuthError("blogsclub_exchange_failed", 502, "BlogsClub login is unavailable")
	}
	request, err = http.NewRequestWithContext(ctx, http.MethodGet, blogsClubEndpoint+"?"+url.Values{"action": {"userinfo"}, "access_token": {token.AccessToken}}.Encode(), nil)
	if err != nil {
		return oauthIdentity{}, err
	}
	var profile struct {
		Status int              `json:"status"`
		Data   blogsClubProfile `json:"data"`
	}
	if err := adapter.read(request, &profile); err != nil || profile.Status != 200 || profile.Data.OpenID != token.OpenID || !numericSubject(profile.Data.OpenID) {
		return oauthIdentity{}, newAuthError("blogsclub_identity_failed", 502, "BlogsClub login is unavailable")
	}
	identity := profile.Data
	email := normalizeEmail(identity.Email)
	if !validEmail(email) {
		return oauthIdentity{}, newAuthError("blogsclub_email_required", 409, "A BlogsClub email is required")
	}
	name := strings.TrimSpace(identity.BlogName)
	if name == "" {
		name = "BlogsClub " + identity.OpenID
	}
	encoded, err := json.Marshal(identity)
	return oauthIdentity{Subject: identity.OpenID, Login: identity.OpenID, Name: name, Email: email, Profile: encoded, BlogURL: identity.BlogURL, Verified: identity.Verified == 1}, err
}

func (adapter blogsClubAdapter) read(request *http.Request, target any) error {
	//nolint:gosec // Both requests are constructed from the fixed BlogsClub HTTPS endpoint.
	response, err := adapter.service.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode/100 != 2 {
		return newAuthError("blogsclub_unavailable", 502, "BlogsClub login is unavailable")
	}
	return json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(target)
}

func numericSubject(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
