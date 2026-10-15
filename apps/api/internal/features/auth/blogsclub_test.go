package auth

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBlogsClubAdapter_when_ExchangingAuthorization(t *testing.T) {
	// Given BlogsClub's ordinary redirect protocol and server-only credentials.
	service := NewService(Dependencies{Config: Config{WebBaseURL: "https://web.example.test", BlogsClubClientID: "client", BlogsClubClientSecret: "secret"}})
	adapter := blogsClubAdapter{service}
	service.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := `{"status":200,"access_token":"token","openid":"42"}`
		if request.URL.Query().Get("action") == "token" {
			require.NoError(t, request.ParseForm())
			require.Equal(t, "secret", request.Form.Get("client_secret"))
			require.Equal(t, "https://web.example.test/auth/blogsclub/callback", request.Form.Get("redirect_uri"))
		} else {
			require.Equal(t, "token", request.URL.Query().Get("access_token"))
			body = `{"status":200,"data":{"openid":"42","blog_name":"Example","verified":1,"blog_url":"https://example.test/blog/","email":"Reader@Example.test"}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	// When the code is exchanged and its current profile is read.
	identity, err := adapter.identity(t.Context(), "code")
	// Then the adapter exposes trusted normalized identity and site proof only.
	require.NoError(t, err)
	require.Equal(t, "42", identity.Subject)
	require.Equal(t, "reader@example.test", identity.Email)
	require.True(t, identity.Verified)
	require.NotContains(t, string(identity.Profile), "token")
}

func TestBlogsClubAdapter_when_ProviderRejectsOrChangesSubject(t *testing.T) {
	for _, response := range []string{`{"status":403,"msg":"secret upstream failure"}`, `{"status":200,"data":{"openid":"99","email":"reader@example.test"}}`, `{"status":200,"data":{"openid":"42","email":""}}`} {
		t.Run(response, func(t *testing.T) {
			// Given a valid token but an unsuccessful or inconsistent profile response.
			service := NewService(Dependencies{})
			service.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				body := response
				if request.URL.Query().Get("action") == "token" {
					body = `{"status":200,"access_token":"token","openid":"42"}`
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			// When authorization is resolved.
			_, err := (blogsClubAdapter{service}).identity(t.Context(), "code")
			// Then no inconsistent identity or upstream diagnostic escapes.
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret upstream")
		})
	}
}

func TestBlogsClubAuthorization_when_UsingPopupBridge(t *testing.T) {
	// Given the shared popup bridge uses the registered callback.
	service := NewService(Dependencies{Config: Config{WebBaseURL: "https://web.example.test", BlogsClubClientID: "client", BlogsClubClientSecret: "secret"}})
	// When an authorization URL is created.
	target, err := (blogsClubAdapter{service}).authorizationURL("state")
	// Then it returns through our callback rather than the provider's native message mode.
	require.NoError(t, err)
	parsed, err := url.Parse(target)
	require.NoError(t, err)
	require.Empty(t, parsed.Query().Get("display"))
	require.Equal(t, "basic,email", parsed.Query().Get("scope"))
	require.Equal(t, "https://web.example.test/auth/blogsclub/callback", parsed.Query().Get("redirect_uri"))
	require.Empty(t, parsed.Query().Get("client_secret"))
}
