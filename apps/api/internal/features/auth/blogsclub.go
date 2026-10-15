package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"heyblog-api/internal/platform/apperror"
)

const blogsClubEndpoint = "https://www.blogsclub.org/Oauth/getOauthData"

type blogsClubAdapter struct{ service *Service }

type blogsClubOAuthConfig struct {
	clientID     string
	clientSecret string
	redirectURI  string
}

type blogsClubProfile struct {
	OpenID   blogsClubSubject `json:"openid"`
	BlogName string           `json:"blog_name"`
	Avatar   string           `json:"blogger_avatar"`
	Verified int              `json:"verified"`
	BlogURL  string           `json:"blog_url"`
	Email    string           `json:"email"`
}

type blogsClubResponseInfo struct {
	phase string
}

func (info blogsClubResponseInfo) failure(cause error) error {
	code := "blogsclub_identity_failed"
	if info.phase == "token" {
		code = "blogsclub_exchange_failed"
	}
	return apperror.Wrap(cause, apperror.KindUnavailable, code, "BlogsClub login is unavailable", "read BlogsClub "+info.phase)
}

func (info blogsClubResponseInfo) validateStatus(status *int) error {
	if status == nil || *status != http.StatusOK {
		return info.failure(nil)
	}
	return nil
}

// BlogsClub returns openid as either a JSON string or number depending on the
// endpoint. Normalize both representations before comparing the token and
// profile subjects.
type blogsClubSubject string

func (subject *blogsClubSubject) UnmarshalJSON(raw []byte) error {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	switch value := value.(type) {
	case string:
		*subject = blogsClubSubject(value)
		return nil
	case json.Number:
		*subject = blogsClubSubject(value.String())
		return nil
	default:
		return &json.UnmarshalTypeError{Value: fmt.Sprintf("%T", value), Type: reflect.TypeOf("")}
	}
}

func (adapter blogsClubAdapter) callbackURL() string {
	return strings.TrimRight(adapter.service.config.WebBaseURL, "/") + "/auth/blogsclub/callback"
}

func (adapter blogsClubAdapter) oauthConfig() (blogsClubOAuthConfig, error) {
	config := adapter.service.config
	clientID := strings.TrimSpace(config.BlogsClubClientID)
	clientSecret := strings.TrimSpace(config.BlogsClubClientSecret)
	if clientID == "" || clientSecret == "" {
		return blogsClubOAuthConfig{}, newAuthError("blogsclub_unavailable", 503, "BlogsClub login is unavailable")
	}
	return blogsClubOAuthConfig{clientID: clientID, clientSecret: clientSecret, redirectURI: adapter.callbackURL()}, nil
}

func (config blogsClubOAuthConfig) authorizationURL(state string) string {
	return "https://www.blogsclub.org/Oauth/Login?" + url.Values{
		"client_id":     {config.clientID},
		"redirect_uri":  {config.redirectURI},
		"response_type": {"code"},
		"scope":         {"basic,email"},
		"state":         {state},
	}.Encode()
}

func (config blogsClubOAuthConfig) tokenForm(code string) url.Values {
	return url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {config.redirectURI},
		"client_id":     {config.clientID},
		"client_secret": {config.clientSecret},
	}
}

func (adapter blogsClubAdapter) authorizationURL(state string) (string, error) {
	config, err := adapter.oauthConfig()
	if err != nil {
		return "", err
	}
	return config.authorizationURL(state), nil
}

func (adapter blogsClubAdapter) identity(ctx context.Context, code string) (oauthIdentity, error) {
	config, err := adapter.oauthConfig()
	if err != nil {
		return oauthIdentity{}, err
	}
	form := config.tokenForm(code)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, blogsClubEndpoint+"?action=token", strings.NewReader(form.Encode()))
	if err != nil {
		return oauthIdentity{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var token struct {
		Status      *int             `json:"status"`
		AccessToken string           `json:"access_token"`
		OpenID      blogsClubSubject `json:"openid"`
	}
	info, err := adapter.read(request, &token, "token")
	if err != nil {
		return oauthIdentity{}, err
	}
	if err := info.validateStatus(token.Status); err != nil {
		return oauthIdentity{}, err
	}
	if token.AccessToken == "" {
		return oauthIdentity{}, info.failure(nil)
	}
	if token.OpenID == "" {
		return oauthIdentity{}, info.failure(nil)
	}
	request, err = http.NewRequestWithContext(ctx, http.MethodGet, blogsClubEndpoint+"?"+url.Values{"action": {"userinfo"}, "access_token": {token.AccessToken}}.Encode(), nil)
	if err != nil {
		return oauthIdentity{}, err
	}
	var profile struct {
		Status *int             `json:"status"`
		Data   blogsClubProfile `json:"data"`
	}
	info, err = adapter.read(request, &profile, "userinfo")
	if err != nil {
		return oauthIdentity{}, err
	}
	if err := info.validateStatus(profile.Status); err != nil {
		return oauthIdentity{}, err
	}
	if profile.Data.OpenID == "" {
		return oauthIdentity{}, info.failure(nil)
	}
	if profile.Data.OpenID != token.OpenID {
		return oauthIdentity{}, info.failure(nil)
	}
	if !numericSubject(string(profile.Data.OpenID)) {
		return oauthIdentity{}, info.failure(nil)
	}
	identity := profile.Data
	email := normalizeEmail(identity.Email)
	if !validEmail(email) {
		return oauthIdentity{}, newAuthError("blogsclub_email_required", 409, "A BlogsClub email is required")
	}
	name := strings.TrimSpace(identity.BlogName)
	if name == "" {
		name = "BlogsClub " + string(identity.OpenID)
	}
	encoded, err := json.Marshal(identity)
	subject := string(identity.OpenID)
	return oauthIdentity{Subject: subject, Login: subject, Name: name, Email: email, Profile: encoded, BlogURL: identity.BlogURL, Verified: identity.Verified == 1}, err
}

func (adapter blogsClubAdapter) read(request *http.Request, target any, phase string) (blogsClubResponseInfo, error) {
	info := blogsClubResponseInfo{phase: phase}
	request.Header.Set("User-Agent", "HeyBlog-OAuth")
	//nolint:gosec // Both requests are constructed from the fixed BlogsClub HTTPS endpoint.
	response, err := adapter.service.httpClient.Do(request)
	if err != nil {
		return info, info.failure(err)
	}
	defer func() { _ = response.Body.Close() }()
	rawBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return info, info.failure(err)
	}
	if response.StatusCode/100 != 2 {
		return info, info.failure(nil)
	}
	if err := json.Unmarshal(rawBody, target); err != nil {
		return info, info.failure(err)
	}
	return info, nil
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
