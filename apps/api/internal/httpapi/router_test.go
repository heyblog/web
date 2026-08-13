package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"heyblog-api/internal/config"
)

const testHealthcheckToken = "test-healthcheck-token-0123456789abcdef"
const testWebToken = "test-web-service-token-0123456789abcdef"

func TestRouterKeepsPingContractAndAddsHealthEndpoints(t *testing.T) {
	t.Parallel()

	health := NewHealth(readinessFunc(func(context.Context) error { return nil }), time.Second)
	router := newTestRouter(t, health)

	tests := []struct {
		path       string
		wantStatus int
		wantBody   string
	}{
		{path: "/ping", wantStatus: http.StatusOK, wantBody: `{"message":"pong"}`},
		{path: "/health/live", wantStatus: http.StatusNoContent},
		{path: "/health/ready", wantStatus: http.StatusNoContent},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			if strings.HasPrefix(test.path, "/health/") {
				request.Header.Set("Authorization", "Bearer "+testHealthcheckToken)
			} else {
				request.Header.Set(WebTokenHeader, testWebToken)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.wantStatus || response.Body.String() != test.wantBody {
				t.Fatalf("response = (%d, %q), want (%d, %q)", response.Code, response.Body.String(), test.wantStatus, test.wantBody)
			}
			if strings.HasPrefix(test.path, "/health/") && response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("Cache-Control = %q, want no-store", response.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestWebEndpointRequiresServiceToken(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t, NewHealth(readinessFunc(func(context.Context) error { return nil }), time.Second))
	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{name: "missing", wantStatus: http.StatusUnauthorized},
		{name: "invalid", token: "invalid-web-service-token-0123456789", wantStatus: http.StatusUnauthorized}, // #nosec G101 -- this is a non-functional test token.
		{name: "valid", token: testWebToken, wantStatus: http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/ping", nil)
			request.Header.Set(WebTokenHeader, test.token)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
}

func TestPublicEndpointAudienceDoesNotRequireWebToken(t *testing.T) {
	t.Parallel()

	handler, err := adaptApplicationEndpoint(endpointAudiencePublic, testWebToken, func(*Context) (Response, error) {
		return JSON(http.StatusOK, map[string]string{"message": "public"})
	})
	if err != nil {
		t.Fatalf("adaptApplicationEndpoint() error = %v", err)
	}
	gine := gin.New()
	gine.GET("/public", handler)
	response := httptest.NewRecorder()
	gine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/public", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestProblemInstancePreservesEscapedPath(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t, NewHealth(readinessFunc(func(context.Context) error { return nil }), time.Second))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/missing%20item", nil))

	if !strings.Contains(response.Body.String(), `"instance":"/missing%20item"`) {
		t.Fatalf("body = %q, want escaped problem instance", response.Body.String())
	}
}

func TestReadinessFailureReturnsSafeServiceUnavailableProblem(t *testing.T) {
	t.Parallel()

	health := NewHealth(readinessFunc(func(context.Context) error {
		return errors.New("redis://user:secret@example.test")
	}), time.Second)
	router := newTestRouter(t, health)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	request.Header.Set("Authorization", "Bearer "+testHealthcheckToken)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), `"code":"service_unavailable"`) {
		t.Fatalf("response = (%d, %q), want service unavailable problem", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "secret") {
		t.Fatalf("response leaked readiness details: %q", response.Body.String())
	}
}

func TestReadinessStopsWhenDraining(t *testing.T) {
	t.Parallel()

	checkCalled := false
	health := NewHealth(readinessFunc(func(context.Context) error {
		checkCalled = true
		return nil
	}), time.Second)
	health.BeginDrain()

	if err := health.Ready(context.Background()); err == nil {
		t.Fatal("Ready() error = nil while draining")
	}
	if checkCalled {
		t.Fatal("dependency readiness was checked after draining began")
	}
}

func TestHealthEndpointsRequireBearerToken(t *testing.T) {
	t.Parallel()

	var readinessCalls atomic.Int32
	health := NewHealth(readinessFunc(func(context.Context) error {
		readinessCalls.Add(1)
		return nil
	}), time.Second)
	router := newTestRouter(t, health)

	tests := map[string]string{
		"missing": "",
		"invalid": "Bearer invalid-healthcheck-token-0123456789",
	}
	for _, path := range []string{"/health/live", "/health/ready"} {
		for name, authorization := range tests {
			t.Run(path+"/"+name, func(t *testing.T) {
				request := httptest.NewRequest(http.MethodGet, path, nil)
				if authorization != "" {
					request.Header.Set("Authorization", authorization)
				}
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)

				if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), `"code":"unauthorized"`) {
					t.Fatalf("response = (%d, %q), want unauthorized problem", response.Code, response.Body.String())
				}
				if response.Header().Get("WWW-Authenticate") != `Bearer realm="heyblog-health"` {
					t.Fatalf("WWW-Authenticate = %q, want Bearer challenge", response.Header().Get("WWW-Authenticate"))
				}
			})
		}
	}
	if readinessCalls.Load() != 0 {
		t.Fatalf("readiness calls = %d, want 0", readinessCalls.Load())
	}
}

func TestRouterReturnsProblemsForUnknownRouteAndMethod(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t, NewHealth(readinessFunc(func(context.Context) error { return nil }), time.Second))
	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/missing", nil),
		httptest.NewRequest(http.MethodPost, "/ping", nil),
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound && response.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want 404 or 405", response.Code)
		}
		if response.Header().Get("Content-Type") != ProblemMediaType {
			t.Fatalf("Content-Type = %q, want %q", response.Header().Get("Content-Type"), ProblemMediaType)
		}
	}
}

func TestRouterValidatesAndEchoesRequestID(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t, NewHealth(readinessFunc(func(context.Context) error { return nil }), time.Second))
	tests := []struct {
		name      string
		input     string
		wantExact string
	}{
		{name: "accepted", input: "client-request.123", wantExact: "client-request.123"},
		{name: "replaced", input: "bad id"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/ping", nil)
			request.Header.Set(RequestIDHeader, test.input)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			got := response.Header().Get(RequestIDHeader)
			if test.wantExact != "" && got != test.wantExact {
				t.Fatalf("request ID = %q, want %q", got, test.wantExact)
			}
			if test.wantExact == "" && !regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(got) {
				t.Fatalf("generated request ID = %q, want 128-bit lowercase hex", got)
			}
		})
	}
}

func TestRecoveryUsesProblemDetailsWithoutPanicValue(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	router := gin.New()
	router.Use(errorBoundary(logger), requestIDMiddleware())
	router.GET("/panic", func(*gin.Context) { panic("private panic value") })
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if response.Code != http.StatusInternalServerError || response.Header().Get("Content-Type") != ProblemMediaType {
		t.Fatalf("response = (%d, %q), want internal problem", response.Code, response.Header().Get("Content-Type"))
	}
	if strings.Contains(response.Body.String(), "private panic value") || strings.Contains(logs.String(), "private panic value") {
		t.Fatal("panic value leaked to response or logs")
	}
	if !strings.Contains(logs.String(), "stack") {
		t.Fatal("panic log omitted stack")
	}
}

func TestBodyLimitMapsReadFailureToRequestTooLarge(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := gin.New()
	router.Use(errorBoundary(logger), requestIDMiddleware(), bodyLimitMiddleware(4))
	router.POST("/body", Adapt(func(ctx *Context) (Response, error) {
		_, err := io.ReadAll(ctx.Request.Body)
		if err != nil {
			return Response{}, err
		}
		return NoContent(http.StatusNoContent), nil
	}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/body", strings.NewReader("12345")))

	if response.Code != http.StatusRequestEntityTooLarge || !strings.Contains(response.Body.String(), `"code":"request_too_large"`) {
		t.Fatalf("response = (%d, %q), want request-too-large problem", response.Code, response.Body.String())
	}
}

func TestBodyLimitRejectsDeclaredOversizedBodyBeforeHandler(t *testing.T) {
	t.Parallel()

	router := newRouterWithConfig(t, config.HTTPConfig{
		MaxBodyBytes:   4,
		TrustedProxies: []string{},
	}, NewHealth(readinessFunc(func(context.Context) error { return nil }), time.Second), io.Discard)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/ping", strings.NewReader("12345")))

	if response.Code != http.StatusRequestEntityTooLarge || !strings.Contains(response.Body.String(), `"code":"request_too_large"`) {
		t.Fatalf("response = (%d, %q), want request-too-large problem", response.Code, response.Body.String())
	}
}

func TestCORSAndSecurityHeaders(t *testing.T) {
	t.Parallel()

	configuration := testHTTPConfig()
	configuration.CORS.AllowOrigins = []string{"https://web.example.test"}
	router := newRouterWithConfig(t, configuration, NewHealth(readinessFunc(func(context.Context) error { return nil }), time.Second), io.Discard)

	request := httptest.NewRequest(http.MethodOptions, "/ping", nil)
	request.Header.Set("Origin", "https://web.example.test")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	request.Header.Set("Access-Control-Request-Headers", "Authorization, X-Request-ID")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent || response.Header().Get("Access-Control-Allow-Origin") != "https://web.example.test" {
		t.Fatalf("preflight response = (%d, %v), want allowed origin", response.Code, response.Header())
	}
	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	} {
		if got := response.Header().Get(header); got != want {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestCORSRejectsUnlistedOriginWithProblem(t *testing.T) {
	t.Parallel()

	configuration := testHTTPConfig()
	configuration.CORS.AllowOrigins = []string{"https://web.example.test"}
	router := newRouterWithConfig(t, configuration, NewHealth(readinessFunc(func(context.Context) error { return nil }), time.Second), io.Discard)
	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	request.Header.Set("Origin", "https://attacker.example.test")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden || response.Header().Get("Content-Type") != ProblemMediaType || !strings.Contains(response.Body.String(), `"code":"forbidden"`) {
		t.Fatalf("response = (%d, %q, %q), want forbidden problem", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}
}

func TestAccessLogDoesNotIncludeQueryString(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	router := newRouterWithConfig(t, testHTTPConfig(), NewHealth(readinessFunc(func(context.Context) error { return nil }), time.Second), &logs)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ping?token=private-value", nil))

	if strings.Contains(logs.String(), "private-value") || !strings.Contains(logs.String(), `request_id`) {
		t.Fatalf("access log = %q, want request ID without query value", logs.String())
	}
}

func newTestRouter(t *testing.T, health *Health) *gin.Engine {
	t.Helper()
	return newRouterWithConfig(t, testHTTPConfig(), health, io.Discard)
}

func newRouterWithConfig(t *testing.T, configuration config.HTTPConfig, health *Health, output io.Writer) *gin.Engine {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(output, nil))
	router, err := NewRouter(Options{
		Mode:             config.ModeDevelopment,
		HTTP:             configuration,
		Logger:           logger,
		Health:           health,
		HealthcheckToken: testHealthcheckToken,
		WebToken:         testWebToken,
	})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	return router
}

func testHTTPConfig() config.HTTPConfig {
	return config.HTTPConfig{
		MaxBodyBytes:   1 << 20,
		TrustedProxies: []string{},
		CORS: config.CORSConfig{
			AllowOrigins:     []string{},
			AllowCredentials: true,
		},
	}
}

type readinessFunc func(context.Context) error

func (function readinessFunc) Ready(ctx context.Context) error {
	return function(ctx)
}
