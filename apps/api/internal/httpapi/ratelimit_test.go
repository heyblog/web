package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"heyblog-api/internal/ratelimit"
)

func TestRateLimitMiddlewareStopsDeniedRequest(t *testing.T) {
	t.Parallel()

	endpointCalled := false
	limiter := limiterFunc(func(context.Context, string, ratelimit.Policy) (ratelimit.Decision, error) {
		return ratelimit.Decision{
			Allowed:    false,
			Limit:      10,
			Remaining:  0,
			RetryAfter: 1500 * time.Millisecond,
			ResetAfter: 5 * time.Second,
		}, nil
	})
	router := rateLimitTestRouter(t, limiter, func(*Context) (Response, error) {
		endpointCalled = true
		return NoContent(http.StatusNoContent), nil
	})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/limited", nil))

	if response.Code != http.StatusTooManyRequests || !strings.Contains(response.Body.String(), `"code":"rate_limited"`) {
		t.Fatalf("response = (%d, %q), want rate-limited problem", response.Code, response.Body.String())
	}
	if endpointCalled {
		t.Fatal("endpoint ran after rate limiter denied the request")
	}
	if response.Header().Get("Retry-After") != "2" || response.Header().Get("RateLimit-Limit") != "10" || response.Header().Get("RateLimit-Remaining") != "0" || response.Header().Get("RateLimit-Reset") != "5" {
		t.Fatalf("rate limit headers = %v, want rounded retry/reset metadata", response.Header())
	}
}

func TestRateLimitMiddlewareFailsClosed(t *testing.T) {
	t.Parallel()

	endpointCalled := false
	limiter := limiterFunc(func(context.Context, string, ratelimit.Policy) (ratelimit.Decision, error) {
		return ratelimit.Decision{}, errors.New("redis unavailable")
	})
	router := rateLimitTestRouter(t, limiter, func(*Context) (Response, error) {
		endpointCalled = true
		return NoContent(http.StatusNoContent), nil
	})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/limited", nil))

	if response.Code != http.StatusServiceUnavailable || endpointCalled {
		t.Fatalf("response status = %d, endpointCalled = %t; want fail-closed 503", response.Code, endpointCalled)
	}
}

func TestEnforceRateLimitReturnsRetryMetadataForDeniedIdentifier(t *testing.T) {
	t.Parallel()

	limiter := limiterFunc(func(context.Context, string, ratelimit.Policy) (ratelimit.Decision, error) {
		return ratelimit.Decision{
			Allowed:    false,
			Limit:      1,
			Remaining:  0,
			RetryAfter: 59 * time.Second,
			ResetAfter: time.Minute,
		}, nil
	})
	router := gin.New()
	router.Use(errorBoundary(slog.New(slog.NewTextHandler(io.Discard, nil))))
	router.GET("/limited", Adapt(func(ctx *Context) (Response, error) {
		if err := EnforceRateLimit(ctx, limiter, "reader@example.test", ratelimit.Policy{
			Name: "mail", Capacity: 1, RefillTokens: 1, RefillInterval: time.Minute,
		}); err != nil {
			return Response{}, err
		}
		return NoContent(http.StatusNoContent), nil
	}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/limited", nil))

	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "59" {
		t.Fatalf("response = (%d, %v), want 429 with retry metadata", response.Code, response.Header())
	}
}

func rateLimitTestRouter(t *testing.T, limiter RateLimiter, endpoint Endpoint) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	if err := router.SetTrustedProxies(nil); err != nil {
		t.Fatalf("SetTrustedProxies() error = %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router.Use(errorBoundary(logger), requestIDMiddleware())
	policy := ratelimit.Policy{Name: "test", Capacity: 10, RefillTokens: 1, RefillInterval: time.Second}
	router.GET("/limited", Adapt(Chain(endpoint, RateLimit(limiter, policy))))
	return router
}

type limiterFunc func(context.Context, string, ratelimit.Policy) (ratelimit.Decision, error)

func (function limiterFunc) Allow(ctx context.Context, key string, policy ratelimit.Policy) (ratelimit.Decision, error) {
	return function(ctx, key, policy)
}
