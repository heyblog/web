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

	"heyblog-api/internal/apperror"
	"heyblog-api/internal/application/publicview"
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

func TestPublicViewRoutesUseWebAuthenticationAndTypedReader(t *testing.T) {
	t.Parallel()

	var gotIdentifier publicview.SiteIdentifier
	var gotCustomID string
	views := publicViewReaderStub{
		home: func(context.Context) (publicview.Home, error) {
			return publicview.Home{SiteCount: 2, Sites: []publicview.HomeSiteCard{}}, nil
		},
		directory: func(
			_ context.Context,
			query publicview.DirectoryQuery,
		) (publicview.DirectoryView, error) {
			return publicview.DirectoryView{
				Items: []publicview.SiteCardView{}, Query: query,
				Pagination: publicview.DirectoryPagination{Page: 1, PageSize: 24, TotalPages: 1},
			}, nil
		},
		directoryOptions: func(context.Context) (publicview.DirectoryOptions, error) {
			return publicview.DirectoryOptions{
				PrimaryTags: []publicview.DirectoryOption{{
					Value: "technology", Label: "技术", NormalCount: 2,
				}},
				SecondaryTags: []publicview.DirectoryOption{}, Warnings: []publicview.DirectoryOption{},
				Technologies: []publicview.DirectoryOption{},
			}, nil
		},
		byIdentifier: func(_ context.Context, identifier publicview.SiteIdentifier) (publicview.SiteProfile, error) {
			gotIdentifier = identifier
			return publicview.SiteProfile{SiteCard: publicview.SiteCard{ShortID: identifier.Value}}, nil
		},
		byCustomID: func(_ context.Context, customID string) (publicview.SiteProfile, error) {
			gotCustomID = customID
			return publicview.SiteProfile{SiteCard: publicview.SiteCard{CustomID: &customID}}, nil
		},
	}
	router := newRouterWithViews(t, testHTTPConfig(), views)

	tests := []struct {
		path       string
		wantStatus int
		wantBody   string
	}{
		{path: "/home", wantStatus: http.StatusOK, wantBody: `"siteCount":2`},
		{path: "/sites?q=Astro", wantStatus: http.StatusOK, wantBody: `"q":"Astro"`},
		{path: "/sites/options", wantStatus: http.StatusOK, wantBody: `"value":"technology"`},
		{path: "/sites/id/A1b2C3d4E", wantStatus: http.StatusOK, wantBody: `"shortId":"A1b2C3d4E"`},
		{path: "/sites/custom/My_Blog", wantStatus: http.StatusOK, wantBody: `"customId":"My_Blog"`},
	}
	for _, test := range tests {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		request.Header.Set(WebTokenHeader, testWebToken)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != test.wantStatus || !strings.Contains(response.Body.String(), test.wantBody) {
			t.Fatalf("%s response = (%d, %q), want status %d containing %q", test.path, response.Code, response.Body.String(), test.wantStatus, test.wantBody)
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s Cache-Control = %q, want no-store", test.path, response.Header().Get("Cache-Control"))
		}
	}
	if gotIdentifier.Kind != publicview.IdentifierShortID || gotIdentifier.Value != "A1b2C3d4E" {
		t.Fatalf("identifier = %#v", gotIdentifier)
	}
	if gotCustomID != "My_Blog" {
		t.Fatalf("custom ID = %q", gotCustomID)
	}

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/home", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}
}

func TestPublicViewRoutesRejectInvalidIdentifiersBeforeApplication(t *testing.T) {
	t.Parallel()

	applicationCalls := 0
	views := publicViewReaderStub{
		byIdentifier: func(context.Context, publicview.SiteIdentifier) (publicview.SiteProfile, error) {
			applicationCalls++
			return publicview.SiteProfile{}, nil
		},
		byCustomID: func(context.Context, string) (publicview.SiteProfile, error) {
			applicationCalls++
			return publicview.SiteProfile{}, nil
		},
	}
	router := newRouterWithViews(t, testHTTPConfig(), views)
	for _, path := range []string{"/sites/id/not-valid", "/sites/custom/a"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set(WebTokenHeader, testWebToken)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"bad_request"`) {
			t.Fatalf("%s response = (%d, %q), want bad request", path, response.Code, response.Body.String())
		}
	}
	if applicationCalls != 0 {
		t.Fatalf("application calls = %d, want 0", applicationCalls)
	}
}

func TestPublicViewRoutePreservesNotFoundProblem(t *testing.T) {
	t.Parallel()

	router := newRouterWithViews(t, testHTTPConfig(), publicViewReaderStub{
		byIdentifier: func(context.Context, publicview.SiteIdentifier) (publicview.SiteProfile, error) {
			return publicview.SiteProfile{}, apperror.New(
				apperror.KindNotFound,
				apperror.CodeNotFound,
				"site was not found",
			)
		},
	})
	request := httptest.NewRequest(http.MethodGet, "/sites/id/A1b2C3d4E", nil)
	request.Header.Set(WebTokenHeader, testWebToken)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
		t.Fatalf("response = (%d, %q), want not found", response.Code, response.Body.String())
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

func TestBodyLimitUsesMethodAndRouteOverride(t *testing.T) {
	t.Parallel()

	configuration := testHTTPConfig()
	configuration.MaxBodyBytes = 4
	router, err := NewRouter(Options{
		Mode:             config.ModeDevelopment,
		HTTP:             configuration,
		Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		HealthcheckToken: testHealthcheckToken,
		WebToken:         testWebToken,
		PublicViews:      publicViewReaderStub{},
		BodyLimitOverrides: map[Route]int64{
			{Method: http.MethodPost, Path: "/large"}: 8,
		},
	})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	router.POST("/large", Adapt(func(ctx *Context) (Response, error) {
		if _, err := io.ReadAll(ctx.Request.Body); err != nil {
			return Response{}, err
		}
		return NoContent(http.StatusNoContent), nil
	}))

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/large", strings.NewReader("12345678")))
	if response.Code != http.StatusNoContent {
		t.Fatalf("override status = %d, want %d", response.Code, http.StatusNoContent)
	}

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/large", strings.NewReader("12345")))
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("default status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
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
	return newRouterWithDependencies(t, configuration, health, output, publicViewReaderStub{})
}

func newRouterWithViews(
	t *testing.T,
	configuration config.HTTPConfig,
	views publicview.Reader,
) *gin.Engine {
	t.Helper()
	return newRouterWithDependencies(
		t,
		configuration,
		NewHealth(readinessFunc(func(context.Context) error { return nil }), time.Second),
		io.Discard,
		views,
	)
}

func newRouterWithDependencies(
	t *testing.T,
	configuration config.HTTPConfig,
	health *Health,
	output io.Writer,
	views publicview.Reader,
) *gin.Engine {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(output, nil))
	router, err := NewRouter(Options{
		Mode:             config.ModeDevelopment,
		HTTP:             configuration,
		Logger:           logger,
		Health:           health,
		HealthcheckToken: testHealthcheckToken,
		WebToken:         testWebToken,
		PublicViews:      views,
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

type publicViewReaderStub struct {
	home             func(context.Context) (publicview.Home, error)
	directory        func(context.Context, publicview.DirectoryQuery) (publicview.DirectoryView, error)
	directoryOptions func(context.Context) (publicview.DirectoryOptions, error)
	byIdentifier     func(context.Context, publicview.SiteIdentifier) (publicview.SiteProfile, error)
	byCustomID       func(context.Context, string) (publicview.SiteProfile, error)
}

func (stub publicViewReaderStub) Home(ctx context.Context) (publicview.Home, error) {
	if stub.home != nil {
		return stub.home(ctx)
	}
	return publicview.Home{Sites: []publicview.HomeSiteCard{}}, nil
}

func (stub publicViewReaderStub) Directory(
	ctx context.Context,
	query publicview.DirectoryQuery,
) (publicview.DirectoryView, error) {
	if stub.directory != nil {
		return stub.directory(ctx, query)
	}
	return publicview.DirectoryView{Items: []publicview.HomeSiteCard{}}, nil
}

func (stub publicViewReaderStub) DirectoryOptions(
	ctx context.Context,
) (publicview.DirectoryOptions, error) {
	if stub.directoryOptions != nil {
		return stub.directoryOptions(ctx)
	}
	return publicview.DirectoryOptions{
		PrimaryTags: []publicview.DirectoryOption{}, SecondaryTags: []publicview.DirectoryOption{},
		Warnings: []publicview.DirectoryOption{}, Technologies: []publicview.DirectoryOption{},
	}, nil
}

func (stub publicViewReaderStub) SiteByIdentifier(
	ctx context.Context,
	identifier publicview.SiteIdentifier,
) (publicview.SiteProfile, error) {
	if stub.byIdentifier != nil {
		return stub.byIdentifier(ctx, identifier)
	}
	return publicview.SiteProfile{}, nil
}

func (stub publicViewReaderStub) SiteByCustomID(
	ctx context.Context,
	customID string,
) (publicview.SiteProfile, error) {
	if stub.byCustomID != nil {
		return stub.byCustomID(ctx, customID)
	}
	return publicview.SiteProfile{}, nil
}
