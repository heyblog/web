package auth

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func blogsClubTestService() *Service {
	return NewService(Dependencies{Config: Config{
		WebBaseURL: "https://web.example.test", BlogsClubClientID: "client", BlogsClubClientSecret: "secret",
	}})
}

func TestBlogsClubAdapter_when_ExchangingAuthorization(t *testing.T) {
	service := blogsClubTestService()
	adapter := blogsClubAdapter{service}
	service.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		require.Equal(t, "HeyBlog-OAuth", request.Header.Get("User-Agent"))
		body := `{"status":200,"access_token":"token","openid":42}`
		if request.URL.Query().Get("action") == "token" {
			require.NoError(t, request.ParseForm())
			require.Equal(t, "authorization_code", request.Form.Get("grant_type"))
			require.Equal(t, "client", request.Form.Get("client_id"))
			require.Equal(t, "secret", request.Form.Get("client_secret"))
			require.Equal(t, "https://web.example.test/auth/blogsclub/callback", request.Form.Get("redirect_uri"))
		} else {
			require.Equal(t, "token", request.URL.Query().Get("access_token"))
			body = `{"status":200,"data":{"openid":42,"blog_name":"Example","verified":1,"blog_url":"https://example.test/blog/","email":"Reader@Example.test"}}`
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}

	identity, err := adapter.identity(t.Context(), "code")
	require.NoError(t, err)
	require.Equal(t, "42", identity.Subject)
	require.Equal(t, "reader@example.test", identity.Email)
	require.True(t, identity.Verified)
	require.NotContains(t, string(identity.Profile), "token")
}

//nolint:gosec // Test payloads intentionally exercise OAuth protocol fields.
func TestBlogsClubAdapter_when_ProviderResponseIsInvalid(t *testing.T) {
	for _, test := range []struct {
		name, tokenBody, profileBody, wantCode string
	}{
		{name: "transport", wantCode: "blogsclub_exchange_failed"},
		{name: "http rejection", tokenBody: `{"status":403}`, wantCode: "blogsclub_exchange_failed"},
		{name: "malformed JSON", tokenBody: `not-json`, wantCode: "blogsclub_exchange_failed"},
		{name: "missing token", tokenBody: `{"status":200,"openid":"42"}`, wantCode: "blogsclub_exchange_failed"},
		{name: "subject mismatch", tokenBody: `{"status":200,"access_token":"token","openid":"42"}`, profileBody: `{"status":200,"data":{"openid":"99","email":"reader@example.test"}}`, wantCode: "blogsclub_identity_failed"},
		{name: "missing email", tokenBody: `{"status":200,"access_token":"token","openid":"42"}`, profileBody: `{"status":200,"data":{"openid":"42"}}`, wantCode: "blogsclub_email_required"},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := blogsClubTestService()
			service.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if test.name == "transport" {
					return nil, errors.New("upstream transport failure")
				}
				body := test.tokenBody
				if request.URL.Query().Get("action") == "userinfo" {
					body = test.profileBody
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			_, err := (blogsClubAdapter{service}).identity(t.Context(), "code")
			require.Equal(t, test.wantCode, oauthErrorCode(err))
		})
	}
}

func TestBlogsClubAuthorization_when_UsingPopupBridge(t *testing.T) {
	service := blogsClubTestService()
	target, err := (blogsClubAdapter{service}).authorizationURL("state")
	require.NoError(t, err)
	parsed, err := url.Parse(target)
	require.NoError(t, err)
	require.Empty(t, parsed.Query().Get("display"))
	require.Equal(t, "basic,email", parsed.Query().Get("scope"))
	require.Equal(t, "https://web.example.test/auth/blogsclub/callback", parsed.Query().Get("redirect_uri"))
	require.Empty(t, parsed.Query().Get("client_secret"))
}
