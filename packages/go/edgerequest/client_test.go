package edgerequest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type resolver struct{}

func (resolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
}

type authorizer struct {
	calls  atomic.Int64
	revoke bool
}

func (a *authorizer) Authorize(_ context.Context, _ Provider, _ string, url string, headers http.Header) (http.Header, error) {
	call := a.calls.Add(1)
	if a.revoke && call > 1 {
		return nil, errors.New("revoked")
	}
	headers.Set("X-HeyBlog-Proof", fmt.Sprintf("fresh-%d-%s", call, url))
	return headers, nil
}
func wireClient(t *testing.T, handler http.HandlerFunc) (*Client, *authorizer) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	authorizer := &authorizer{}
	return New(Config{Endpoints: map[Provider]Endpoint{Cloudflare: {URL: server.URL, Token: "service-secret"}}, Resolver: resolver{}, Authorizer: authorizer}), authorizer
}
func probe() Request {
	return Request{URL: "https://example.com/start", Method: http.MethodGet, Platform: Cloudflare, Headers: http.Header{"User-Agent": []string{"Custom-UA"}}, MaxBytes: 512, Timeout: time.Second, MaxRedirects: 3, SameOrigin: true}
}
func TestExecute_when_RedirectNeedsFreshAuthorization(t *testing.T) {
	// Given an edge endpoint returning a redirect then an empty conditional success.
	var targets []wireRequest
	client, auth := wireClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer service-secret", r.Header.Get("Authorization"))
		var target wireRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&target))
		targets = append(targets, target)
		require.Empty(t, target.Headers["Authorization"])
		require.Equal(t, "Custom-UA", target.Headers["User-Agent"])
		if len(targets) == 1 {
			require.NoError(t, json.NewEncoder(w).Encode(wireResponse{Status: 302, Headers: map[string]string{"location": "/finish"}}))
			return
		}
		require.NoError(t, json.NewEncoder(w).Encode(wireResponse{Status: 304, Node: map[string]string{"colo": "SIN"}}))
	})
	// When the request follows the redirect in Go.
	result, err := client.Execute(t.Context(), probe())
	// Then every hop is separately authorized and 304 does not require a body.
	require.NoError(t, err)
	require.Equal(t, 304, result.Status)
	require.Equal(t, 2, result.Requests)
	require.Equal(t, int64(2), auth.calls.Load())
	require.Equal(t, "https://example.com/finish", result.FinalURL)
	require.NotEqual(t, targets[0].Headers["X-Heyblog-Proof"], targets[1].Headers["X-Heyblog-Proof"])
}
func TestExecute_when_KeyIsRevokedBetweenHops(t *testing.T) {
	// Given a key policy that changes after one request.
	client, auth := wireClient(t, func(w http.ResponseWriter, _ *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(wireResponse{Status: 302, Headers: map[string]string{"location": "/finish"}}))
	})
	auth.revoke = true
	// When a second hop is about to leave the backend.
	result, err := client.Execute(t.Context(), probe())
	// Then the queued snapshot cannot authorize a revoked key.
	var failure *Error
	require.ErrorAs(t, err, &failure)
	require.Equal(t, Signature, failure.Kind)
	require.Equal(t, 1, result.Requests)
}
func TestExecute_when_RedirectTargetsPrivateAddress(t *testing.T) {
	// Given a redirect from a public URL into loopback.
	client, _ := wireClient(t, func(w http.ResponseWriter, _ *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(wireResponse{Status: 302, Headers: map[string]string{"location": "http://127.0.0.1/"}}))
	})
	request := probe()
	request.SameOrigin = false
	// When Go validates the next hop.
	result, err := client.Execute(t.Context(), request)
	// Then no second edge request is dispatched.
	var failure *Error
	require.ErrorAs(t, err, &failure)
	require.Equal(t, Security, failure.Kind)
	require.Equal(t, 1, result.Requests)
}
func TestExecute_when_ProofRedirectChangesOrigin(t *testing.T) {
	// Given a proof response redirecting to another public origin.
	client, auth := wireClient(t, func(w http.ResponseWriter, _ *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(wireResponse{Status: 302, Headers: map[string]string{"location": "https://other.example.com/"}}))
	})
	// When following an origin-bound proof request.
	result, err := client.Execute(t.Context(), probe())
	// Then the other origin is rejected before authorization or dispatch.
	var failure *Error
	require.ErrorAs(t, err, &failure)
	require.Equal(t, Security, failure.Kind)
	require.Equal(t, 1, result.Requests)
	require.EqualValues(t, 1, auth.calls.Load())
}
func TestExecute_when_BudgetIsExhausted(t *testing.T) {
	// Given a denied per-platform reservation.
	client, _ := wireClient(t, func(http.ResponseWriter, *http.Request) { t.Error("budget rejection dispatched a request") })
	client.config.BeforeDispatch = func(context.Context, Provider) error { return errors.New("exhausted") }
	// When a manual or scheduled request is attempted.
	result, err := client.Execute(t.Context(), probe())
	// Then the platform remains untouched and the error is not target health.
	var failure *Error
	require.ErrorAs(t, err, &failure)
	require.Equal(t, Backoff, failure.Kind)
	require.Zero(t, result.Requests)
}
func TestExecute_when_EdgeEnvelopeExceedsLimit(t *testing.T) {
	// Given an authenticated but malformed edge response.
	client, _ := wireClient(t, func(w http.ResponseWriter, _ *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(wireResponse{Status: 200, Body: make([]byte, 513)}))
	})
	// When the response crosses the wire boundary.
	_, err := client.Execute(t.Context(), probe())
	// Then oversized content is classified as a platform protocol failure.
	var failure *Error
	require.ErrorAs(t, err, &failure)
	require.Equal(t, Platform, failure.Kind)
}
