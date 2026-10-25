//go:build integration

package integration_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"heyblog-api/internal/features/auth"
)

func TestPasswordResetSetsFirstPasswordForOAuthAccounts(t *testing.T) {
	fixture := newCredentialFixture(t)
	for _, provider := range []auth.OAuthProvider{auth.ProviderGitHub, auth.ProviderBlogsClub} {
		t.Run(string(provider), func(t *testing.T) {
			ctx := t.Context()
			email := strings.ToLower(string(provider)) + "-reset@example.test"
			recorder := &authMailRecorder{}
			client := &http.Client{Transport: githubRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				body := `{"access_token":"reset-oauth-token"}`
				switch {
				case request.URL.Query().Get("action") == "token":
					body = `{"status":200,"access_token":"reset-oauth-token","openid":"4242"}`
				case request.URL.Query().Get("action") == "userinfo":
					body = fmt.Sprintf(`{"status":200,"data":{"openid":"4242","blog_name":"Reset Blog","verified":1,"blog_url":"https://reset.example.test/","email":%q}}`, email)
				case request.URL.Path == "/user":
					body = `{"id":4242,"login":"password-reset-oauth","name":"Reset OAuth"}`
				case request.URL.Path == "/user/emails":
					body = fmt.Sprintf(`[{"email":%q,"primary":true,"verified":true}]`, email)
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			// #nosec G101 -- Fixed credentials are used only with isolated containers and the fake OAuth client.
			service := auth.NewService(auth.Dependencies{Pool: fixture.pool, Redis: fixture.redis, MailSender: recorder, OAuthHTTPClient: client, Config: auth.Config{
				AccessSecret: "credential-access-test-secret", RefreshSecret: "credential-refresh-test-secret", AccessTTL: time.Hour, RefreshTTL: time.Hour,
				PasswordResetTTL: 30 * time.Minute, WebBaseURL: "https://web.example.test", MailFrom: "verify@example.test",
				GithubClientID: "github-id", GithubClientSecret: "github-secret", GithubScope: "read:user,user:email", BlogsClubClientID: "blogsclub-id", BlogsClubClientSecret: "blogsclub-secret",
			}})
			start, err := service.OAuthStart(ctx, nil, provider, auth.OAuthStartOptions{Next: "/dashboard"})
			require.NoError(t, err)
			request := httptest.NewRequest(http.MethodGet, "https://web.example.test/", nil)
			completion, err := service.OAuthCallback(ctx, request, provider, "reset-code", start.State, start.State, "")
			require.NoError(t, err)
			require.Equal(t, "success", completion.Result.Outcome, completion.Result.ErrorCode)
			request.AddCookie(&http.Cookie{Name: "heyblog_access_token", Value: completion.Cookies[0], Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
			user, err := service.CurrentSession(ctx, request)
			require.NoError(t, err)
			require.False(t, user.HasPassword)
			require.True(t, user.EmailVerified)

			outcome, err := service.ForgotPassword(ctx, email)
			require.NoError(t, err)
			require.Equal(t, auth.PasswordResetSent, outcome)
			require.Len(t, recorder.messages, 1, "verified OAuth account without a password did not receive its first password link")
			token := regexp.MustCompile(`token=([^\s]+)`).FindStringSubmatch(recorder.messages[0].Text)
			require.Len(t, token, 2)
			require.NoError(t, service.ResetPassword(ctx, token[1], "first-local-password"))
			require.Error(t, service.ResetPassword(ctx, token[1], "reused-local-password"))
			_, err = service.CurrentSession(ctx, request)
			require.Error(t, err, "setting a password must invalidate the pre-reset OAuth session")
			user, _, err = service.Login(ctx, email, "first-local-password")
			require.NoError(t, err)
			require.True(t, user.HasPassword)
			require.Contains(t, user.OAuthProviders, provider)
			require.True(t, user.ProfileConfirmationRequired)
		})
	}
}
