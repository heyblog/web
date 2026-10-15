//go:build integration

package integration_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/sitemanagement"
	"heyblog-api/internal/platform/httpapi"
)

func TestOAuthProvidersAndSiteClaims(t *testing.T) {
	fixture := newCredentialFixture(t)
	ctx := t.Context()
	client := &http.Client{Transport: githubRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := ""
		switch {
		case request.URL.Query().Get("action") == "token":
			if err := request.ParseForm(); err != nil {
				return nil, err
			}
			code := request.Form.Get("code")
			if code == "version-change" {
				if _, err := fixture.pool.Exec(ctx, `UPDATE identity.users SET auth_version=auth_version+1 WHERE id=$1::uuid`, fixture.actor); err != nil {
					return nil, err
				}
			}
			body = fmt.Sprintf(`{"status":200,"access_token":%q,"openid":"42"}`, code)
		case request.URL.Query().Get("action") == "userinfo":
			verified, blogURL := 1, "https://oauth-blog.example.test/"
			switch request.URL.Query().Get("access_token") {
			case "mismatch":
				blogURL = "https://other.example.test/"
			case "unverified":
				verified = 0
			}
			body = fmt.Sprintf(`{"status":200,"data":{"openid":"42","blog_name":"OAuth Blog","verified":%d,"blog_url":%q,"email":"credential_admin@example.test"}}`, verified, blogURL)
		case request.URL.Path == "/user":
			body = `{"id":42,"login":"oauth-admin","name":"OAuth Admin"}`
		case request.URL.Path == "/user/emails":
			body = `[{"email":"credential_admin@example.test","primary":true,"verified":true}]`
		default:
			body = `{"access_token":"github-token"}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	service := auth.NewService(auth.Dependencies{Pool: fixture.pool, Redis: fixture.redis, OAuthHTTPClient: client, Config: auth.Config{
		AccessSecret: "credential-access-test-secret", RefreshSecret: "credential-refresh-test-secret", AccessTTL: time.Hour, RefreshTTL: time.Hour, WebBaseURL: "https://web.example.test",
		GithubClientID: "github-id", GithubClientSecret: "github-secret", GithubScope: "read:user,user:email", BlogsClubClientID: "blogsclub-id", BlogsClubClientSecret: "blogsclub-secret",
	}})
	claims := sitemanagement.NewService(sitemanagement.NewRepository(fixture.pool), service, publishedProof{})
	service.SetOAuthClaims(claims)
	require.NoError(t, auth.RegisterRoutes(fixture.router.API, service, credentialWebToken))
	request := func() *http.Request {
		r := httptest.NewRequest("GET", "https://web.example.test/", nil)
		r.AddCookie(&http.Cookie{Name: "heyblog_access_token", Value: fixture.access})
		return r
	}
	start := func(provider auth.OAuthProvider, intent, claimID string) auth.OAuthStartResult {
		result, err := service.OAuthStart(ctx, request(), provider, auth.OAuthStartOptions{Mode: "popup", Intent: intent, ClaimID: claimID, Next: "/dashboard/security"})
		require.NoError(t, err)
		return result
	}
	callback := func(provider auth.OAuthProvider, state auth.OAuthStartResult, code string) auth.OAuthResult {
		result, _, err := service.OAuthCallback(ctx, request(), provider, code, state.State, state.State, "")
		require.NoError(t, err)
		return result
	}
	t.Run("JSON start returns a path-wide HttpOnly cookie", func(t *testing.T) {
		response := httptest.NewRecorder()
		r := request()
		r.URL, _ = url.Parse("/api/web/auth/blogsclub/start?mode=popup")
		r.Header.Set(httpapi.WebTokenHeader, credentialWebToken)
		r.Header.Set("Accept", "application/json")
		fixture.router.ServeHTTP(response, r)
		require.Equal(t, 200, response.Code, response.Body.String())
		var result auth.OAuthStartResult
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
		require.NotEmpty(t, result.AuthorizationURL)
		require.NotEmpty(t, result.State)
		cookies := response.Result().Cookies()
		require.Len(t, cookies, 1)
		require.Equal(t, "/", cookies[0].Path)
		require.True(t, cookies[0].HttpOnly)
	})
	t.Run("navigation start and JSON login callback preserve all cookies", func(t *testing.T) {
		r := request()
		r.URL, _ = url.Parse("/api/web/auth/github/start?mode=redirect")
		r.Header.Set(httpapi.WebTokenHeader, credentialWebToken)
		response := httptest.NewRecorder()
		fixture.router.ServeHTTP(response, r)
		require.Equal(t, 302, response.Code, response.Body.String())
		target, err := url.Parse(response.Header().Get("Location"))
		require.NoError(t, err)
		require.Equal(t, "github.com", target.Host)
		state := start(auth.ProviderBlogsClub, "login", "")
		r = request()
		r.URL, _ = url.Parse("/api/web/auth/blogsclub/callback?code=good&state=" + state.State)
		r.Header.Set(httpapi.WebTokenHeader, credentialWebToken)
		r.Header.Set("Accept", "application/json")
		r.AddCookie(&http.Cookie{Name: "heyblog_oauth_blogsclub_" + state.State, Value: state.State})
		response = httptest.NewRecorder()
		fixture.router.ServeHTTP(response, r)
		require.Equal(t, 200, response.Code, response.Body.String())
		require.Len(t, response.Result().Cookies(), 3)
		var result auth.OAuthResult
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
		require.Equal(t, "success", result.Outcome)
		require.NotContains(t, response.Body.String(), "heyblog_access_token")
	})
	t.Run("concurrent providers associate the same trusted email", func(t *testing.T) {
		states := []auth.OAuthStartResult{start(auth.ProviderGitHub, "login", ""), start(auth.ProviderBlogsClub, "login", "")}
		providers := []auth.OAuthProvider{auth.ProviderGitHub, auth.ProviderBlogsClub}
		var group sync.WaitGroup
		results := make(chan auth.OAuthResult, 2)
		for i := range providers {
			group.Go(func() {
				result, _, err := service.OAuthCallback(ctx, request(), providers[i], "good", states[i].State, states[i].State, "")
				if err != nil {
					t.Error(err)
				}
				results <- result
			})
		}
		group.Wait()
		close(results)
		for result := range results {
			require.Equal(t, "success", result.Outcome, result.ErrorCode)
		}
		user, err := service.Current(ctx, request())
		require.NoError(t, err)
		require.ElementsMatch(t, []auth.OAuthProvider{auth.ProviderGitHub, auth.ProviderBlogsClub}, user.OAuthProviders)
	})
	t.Run("denial consumes state and clears its cookie", func(t *testing.T) {
		state := start(auth.ProviderBlogsClub, "bind", "")
		r := request()
		r.URL, _ = url.Parse("/api/web/auth/blogsclub/callback?error=access_denied&state=" + state.State)
		r.AddCookie(&http.Cookie{Name: "heyblog_oauth_blogsclub_" + state.State, Value: state.State})
		r.Header.Set(httpapi.WebTokenHeader, credentialWebToken)
		r.Header.Set("Accept", "application/json")
		response := httptest.NewRecorder()
		fixture.router.ServeHTTP(response, r)
		require.Equal(t, 200, response.Code, response.Body.String())
		var result auth.OAuthResult
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
		require.Equal(t, "oauth_denied", result.ErrorCode)
		require.Equal(t, "bind", result.Intent)
		cookies := response.Result().Cookies()
		require.Len(t, cookies, 1)
		require.Equal(t, -1, cookies[0].MaxAge)
		require.Equal(t, "/", cookies[0].Path)
		_, _, err := service.OAuthCallback(ctx, request(), auth.ProviderBlogsClub, "good", state.State, state.State, "")
		require.Error(t, err)
	})
	t.Run("provider and cookie mismatch cannot consume another attempt", func(t *testing.T) {
		state := start(auth.ProviderBlogsClub, "bind", "")
		_, _, err := service.OAuthCallback(ctx, request(), auth.ProviderGitHub, "good", state.State, state.State, "")
		require.Error(t, err)
		_, _, err = service.OAuthCallback(ctx, request(), auth.ProviderBlogsClub, "good", state.State, "wrong", "")
		require.Error(t, err)
		require.Equal(t, "success", callback(auth.ProviderBlogsClub, state, "good").Outcome)
	})
	t.Run("site mismatch retains binding and permits fresh retry", func(t *testing.T) {
		_, rotated, err := service.OAuthUnbind(ctx, request(), auth.ProviderBlogsClub)
		require.NoError(t, err)
		fixture.access = rotated[0]
		insertSite(ctx, t, fixture.pool, "OAuth0001", "OAuth Blog", "oauth-blog.example.test")
		actor, err := service.Current(ctx, request())
		require.NoError(t, err)
		created, err := claims.Create(ctx, actor, sitemanagement.SiteClaimCreateInput{ShortID: "OAuth0001", Method: sitemanagement.BlogsClub})
		require.NoError(t, err)
		require.Empty(t, created.Token)
		result := callback(auth.ProviderBlogsClub, start(auth.ProviderBlogsClub, "site-claim", created.Claim.ID), "mismatch")
		require.Equal(t, "blogsclub_site_mismatch", result.ErrorCode)
		bound, err := service.Current(ctx, request())
		require.NoError(t, err)
		require.Contains(t, bound.OAuthProviders, auth.ProviderBlogsClub)
		claim, err := claims.Detail(ctx, actor, created.Claim.ID, false)
		require.NoError(t, err)
		require.Equal(t, "PENDING", claim.Status)
		result = callback(auth.ProviderBlogsClub, start(auth.ProviderBlogsClub, "site-claim", created.Claim.ID), "good")
		require.Equal(t, "success", result.Outcome, result.ErrorCode)
		owner, err := claims.CurrentOwner(ctx, actor, "OAuth0001")
		require.NoError(t, err)
		require.Equal(t, actor.ID, owner.UserID)
	})
	t.Run("a callback cannot bind under a different current account", func(t *testing.T) {
		state := start(auth.ProviderBlogsClub, "bind", "")
		_, err := fixture.pool.Exec(ctx, `INSERT INTO identity.users(email,username,display_name,password_hash,email_verified_at) SELECT 'other-oauth@example.test','oauth_other','Other',password_hash,clock_timestamp() FROM identity.users WHERE id=$1::uuid`, fixture.actor)
		require.NoError(t, err)
		_, tokens, err := service.Login(ctx, "oauth_other", "correct-password")
		require.NoError(t, err)
		other := httptest.NewRequest("GET", "/", nil)
		other.AddCookie(&http.Cookie{Name: "heyblog_access_token", Value: tokens[0]})
		result, _, err := service.OAuthCallback(ctx, other, auth.ProviderBlogsClub, "good", state.State, state.State, "")
		require.NoError(t, err)
		require.Equal(t, "oauth_actor_changed", result.ErrorCode)
		require.Equal(t, "error", result.Outcome)
	})
	t.Run("account changes during exchange prevent final identity write", func(t *testing.T) {
		state := start(auth.ProviderBlogsClub, "bind", "")
		result := callback(auth.ProviderBlogsClub, state, "version-change")
		require.Equal(t, "oauth_actor_changed", result.ErrorCode)
	})
}
