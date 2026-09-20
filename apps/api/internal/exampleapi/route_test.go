package exampleapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"heyblog-api/internal/apikey"
	"heyblog-api/internal/application/publicview"
	"heyblog-api/internal/config"
	"heyblog-api/internal/httpapi"
)

const validExampleToken = "hbk_example_test_token"

func TestExampleRouteReturnsMethodContractForEverySupportedMethod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method     string
		wantStatus int
		wantBody   bool
	}{
		{method: http.MethodGet, wantStatus: http.StatusOK, wantBody: true},
		{method: http.MethodPost, wantStatus: http.StatusOK, wantBody: true},
		{method: http.MethodPut, wantStatus: http.StatusOK, wantBody: true},
		{method: http.MethodPatch, wantStatus: http.StatusOK, wantBody: true},
		{method: http.MethodHead, wantStatus: http.StatusNoContent},
		{method: http.MethodDelete, wantStatus: http.StatusNoContent},
		{method: http.MethodOptions, wantStatus: http.StatusNoContent},
	}
	for _, test := range tests {
		t.Run(test.method, func(t *testing.T) {
			// Given
			router := newExampleTestRouter(t, exampleTestAuthenticator{})
			request := httptest.NewRequest(test.method, Path, nil)
			request.Header.Set("Authorization", "Bearer "+validExampleToken)
			response := httptest.NewRecorder()

			// When
			router.ServeHTTP(response, request)

			// Then
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %q", response.Code, test.wantStatus, response.Body.String())
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("Cache-Control = %q, want no-store", response.Header().Get("Cache-Control"))
			}
			if !test.wantBody {
				if response.Body.Len() != 0 {
					t.Fatalf("body = %q, want empty", response.Body.String())
				}
				return
			}
			var body struct {
				Method string `json:"method"`
				Status string `json:"status"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Method != test.method || body.Status != "ok" {
				t.Fatalf("body = %#v, want method %s and status ok", body, test.method)
			}
		})
	}
}

func TestExampleWriteRoutesDoNotParseRequestBodies(t *testing.T) {
	t.Parallel()

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			// Given
			router := newExampleTestRouter(t, exampleTestAuthenticator{})
			request := httptest.NewRequest(method, Path, strings.NewReader("not-json"))
			request.Header.Set("Authorization", "Bearer "+validExampleToken)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()

			// When
			router.ServeHTTP(response, request)

			// Then
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body = %q", response.Code, http.StatusOK, response.Body.String())
			}
		})
	}
}

func TestExampleOptionsReturnsSupportedMethods(t *testing.T) {
	t.Parallel()

	// Given
	router := newExampleTestRouter(t, exampleTestAuthenticator{})
	request := httptest.NewRequest(http.MethodOptions, Path, nil)
	request.Header.Set("Authorization", "Bearer "+validExampleToken)
	response := httptest.NewRecorder()

	// When
	router.ServeHTTP(response, request)

	// Then
	if got := response.Header().Get("Allow"); got != AllowedMethods {
		t.Fatalf("Allow = %q, want %q", got, AllowedMethods)
	}
}

func TestExampleRouteRejectsMissingInvalidAndInsufficientCredentials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		token      string
		wantStatus int
		wantCode   string
	}{
		{name: "missing key", wantStatus: http.StatusUnauthorized, wantCode: "invalid_api_key"},
		{name: "invalid key", token: "invalid", wantStatus: http.StatusUnauthorized, wantCode: "invalid_api_key"},
		{name: "wrong audience", token: "unknown-audience", wantStatus: http.StatusForbidden, wantCode: "insufficient_scope"},
		{name: "missing scope", token: "missing-scope", wantStatus: http.StatusForbidden, wantCode: "insufficient_scope"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			router := newExampleTestRouter(t, exampleTestAuthenticator{})
			request := httptest.NewRequest(http.MethodGet, Path, nil)
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			response := httptest.NewRecorder()

			// When
			router.ServeHTTP(response, request)

			// Then
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			var problem struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
				t.Fatalf("decode problem: %v", err)
			}
			if problem.Code != test.wantCode {
				t.Fatalf("code = %q, want %q", problem.Code, test.wantCode)
			}
		})
	}
}

func TestExampleRouteLeavesTraceAndConnectUnsupported(t *testing.T) {
	t.Parallel()

	for _, method := range []string{http.MethodTrace, http.MethodConnect} {
		t.Run(method, func(t *testing.T) {
			// Given
			router := newExampleTestRouter(t, exampleTestAuthenticator{})
			request := httptest.NewRequest(method, Path, nil)
			request.Header.Set("Authorization", "Bearer "+validExampleToken)
			response := httptest.NewRecorder()

			// When
			router.ServeHTTP(response, request)

			// Then
			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
			}
			if got := response.Header().Get("Content-Type"); got != "application/problem+json" {
				t.Fatalf("Content-Type = %q, want application/problem+json", got)
			}
		})
	}
}

func newExampleTestRouter(t *testing.T, authenticator apikey.Authenticator) *httpapi.Router {
	t.Helper()
	router, err := httpapi.NewRouter(httpapi.Options{
		Mode:             config.ModeDevelopment,
		HTTP:             config.HTTPConfig{MaxBodyBytes: 1024, TrustedProxies: []string{}, CORS: config.CORSConfig{}},
		HealthcheckToken: "test-healthcheck-token-0123456789abcdef",
		WebToken:         "test-web-service-token-0123456789abcdef",
		PublicViews:      publicview.New(nil),
	})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	RegisterRoutes(router.API, authenticator)
	return router
}

type exampleTestAuthenticator struct{}

func (exampleTestAuthenticator) Authenticate(
	_ context.Context,
	token string,
	policy apikey.AccessPolicy,
) (apikey.Principal, error) {
	if token == "unknown-audience" || token == "missing-scope" {
		return apikey.Principal{}, apikey.ErrInsufficientScope
	}
	if token != validExampleToken || !slices.Contains(policy.Audiences, apikey.AudienceExternal) || policy.Scope != apikey.ScopeExampleCall {
		return apikey.Principal{}, apikey.ErrInvalidToken
	}
	return apikey.Principal{Audience: apikey.AudienceExternal, Scopes: []apikey.Scope{policy.Scope}}, nil
}
