package sitestats

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

	"heyblog-api/internal/features/publicview"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"heyblog-api/internal/platform/config"
	"heyblog-api/internal/platform/httpapi"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type metricStore struct {
	Store
	targetErr  error
	writeErr   error
	writeDelay time.Duration
	calls      int
}

func (store *metricStore) GetSiteOutboundTarget(context.Context, string) (dbgen.GetSiteOutboundTargetRow, error) {
	return dbgen.GetSiteOutboundTargetRow{Scheme: "https", NormalizedHost: "blog.example", BasePath: "/blog"}, store.targetErr
}
func (store *metricStore) RecordSiteMetricEvents(ctx context.Context, _ dbgen.RecordSiteMetricEventsParams) error {
	store.calls++
	if err := waitForMetric(ctx, store.writeDelay); err != nil {
		return err
	}
	return store.writeErr
}

type metricRedis struct {
	redis.Scripter
	err     error
	allowed int64
	delay   time.Duration
}

func (client metricRedis) EvalSha(ctx context.Context, _ string, _ []string, _ ...any) *redis.Cmd {
	if err := waitForMetric(ctx, client.delay); err != nil {
		return redis.NewCmdResult(nil, err)
	}
	return redis.NewCmdResult([]any{client.allowed, int64(239), int64(1000), int64(1000)}, client.err)
}

func waitForMetric(ctx context.Context, delay time.Duration) error {
	if delay == 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func TestOutboundRetainsTargetWhenStatisticsFail(t *testing.T) {
	for _, scenario := range []struct {
		name                   string
		redisErr, writeErr     error
		allowed                int64
		wantCalls              int
		redisDelay, writeDelay time.Duration
		maxDuration            time.Duration
	}{
		{name: "recorded", allowed: 1, wantCalls: 1},
		{name: "metric write failed", allowed: 1, writeErr: errors.New("private database error"), wantCalls: 1},
		{name: "rate limit unavailable", redisErr: errors.New("private redis error")},
		{name: "throttled"},
		{name: "slow limiter", allowed: 1, redisDelay: 2 * time.Second, maxDuration: 1500 * time.Millisecond},
		{name: "limiter and write share budget", allowed: 1, redisDelay: 600 * time.Millisecond, writeDelay: 2 * time.Second, wantCalls: 1, maxDuration: 1500 * time.Millisecond},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			// Given
			store := &metricStore{writeErr: scenario.writeErr, writeDelay: scenario.writeDelay}
			service := &Service{store: store, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			router, err := httpapi.NewRouter(httpapi.Options{WebToken: "test-web", HealthcheckToken: "test-health", PublicViews: publicview.New(nil, nil), HTTP: config.HTTPConfig{MaxBodyBytes: 32_000}})
			if err != nil {
				t.Fatal(err)
			}
			RegisterRoutes(router.API, service, "test-web", metricRedis{err: scenario.redisErr, allowed: scenario.allowed, delay: scenario.redisDelay})
			request := httptest.NewRequest(http.MethodPost, "/sites/id/A1b2C3d4E/outbound", strings.NewReader(`{"eventId":"12345678-1234-4234-9234-123456789012"}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(httpapi.WebTokenHeader, "test-web")
			response := httptest.NewRecorder()
			// When
			started := time.Now()
			router.ServeHTTP(response, request)
			// Then
			if elapsed := time.Since(started); scenario.maxDuration > 0 && elapsed > scenario.maxDuration {
				t.Fatalf("optional statistics delayed navigation for %s, maximum %s", elapsed, scenario.maxDuration)
			}
			if response.Code != 200 || !strings.Contains(response.Body.String(), "https://blog.example/blog") || strings.Contains(response.Body.String(), "private") || store.calls != scenario.wantCalls {
				t.Fatalf("response=%d %s calls=%d", response.Code, response.Body.String(), store.calls)
			}
		})
	}
}

func TestMetricRoutesEnforceAuthenticationAndVisibility(t *testing.T) {
	for _, scenario := range []struct {
		name, token string
		missing     bool
		status      int
	}{
		{name: "unauthenticated", status: 401}, {name: "missing site", token: "test-web", missing: true, status: 404},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			// Given
			store := &metricStore{}
			if scenario.missing {
				store.targetErr = pgx.ErrNoRows
			}
			service := &Service{store: store, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			router, err := httpapi.NewRouter(httpapi.Options{WebToken: "test-web", HealthcheckToken: "test-health", PublicViews: publicview.New(nil, nil), HTTP: config.HTTPConfig{MaxBodyBytes: 32_000}})
			if err != nil {
				t.Fatal(err)
			}
			RegisterRoutes(router.API, service, "test-web", metricRedis{allowed: 1})
			request := httptest.NewRequest(http.MethodPost, "/sites/id/A1b2C3d4E/outbound", strings.NewReader(`{"eventId":null}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(httpapi.WebTokenHeader, scenario.token)
			response := httptest.NewRecorder()
			// When
			router.ServeHTTP(response, request)
			// Then
			if response.Code != scenario.status || store.calls != 0 {
				t.Fatalf("response=%d %s calls=%d", response.Code, response.Body.String(), store.calls)
			}
		})
	}
}
