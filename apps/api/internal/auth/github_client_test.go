package auth

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestGitHubClientExchangesCodeAndReadsVerifiedPrimaryEmail(t *testing.T) {
	service := &Service{config: Config{WebBaseURL: "https://web.example.test", GithubClientID: "client-id", GithubClientSecret: "client-secret"}}
	var redirectURI string
	service.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := `{"access_token":"github-token"}`
		switch request.URL.Path {
		case "/login/oauth/access_token":
			if err := request.ParseForm(); err != nil {
				t.Fatalf("ParseForm() error = %v", err)
			}
			redirectURI = request.Form.Get("redirect_uri")
		case "/user":
			body = `{"id":42,"login":"Octo-Cat","name":"Octo Cat","avatar_url":"https://avatars.example.test/42"}`
		case "/user/emails":
			body = `[{"email":"octo@example.test","primary":true,"verified":true}]`
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	token, err := service.githubAccessToken(context.Background(), "oauth-code")
	if err != nil || token != "github-token" {
		t.Fatalf("githubAccessToken() = (%q, %v)", token, err)
	}
	if redirectURI != "https://web.example.test/auth/github/callback" {
		t.Fatalf("redirect_uri = %q, want Web callback", redirectURI)
	}
	profile, email, err := service.githubIdentity(context.Background(), token)
	if err != nil || profile.ID != 42 || email != "octo@example.test" {
		t.Fatalf("githubIdentity() = (%#v, %q, %v)", profile, email, err)
	}
}

func TestGitHubCallbackURLUsesPublicWebOrigin(t *testing.T) {
	service := &Service{config: Config{WebBaseURL: "https://web.example.test"}}
	if got := service.githubCallbackURL(); got != "https://web.example.test/auth/github/callback" {
		t.Fatalf("githubCallbackURL() = %q, want Web callback", got)
	}
}

func TestGitHubUsernameFitsLocalConstraint(t *testing.T) {
	username := githubUsername(strings.Repeat("a", 80), "12345678901234567890")
	if len(username) > 32 || !usernamePattern.MatchString(username) {
		t.Fatalf("githubUsername() = %q, want valid local username", username)
	}
}
