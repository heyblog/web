package ratelimit

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAllowHashesIdentifierAndMapsDecision(t *testing.T) {
	t.Parallel()

	var capturedKey string
	limiter := newWithEval(func(_ context.Context, keys []string, _ ...any) ([]any, error) {
		capturedKey = keys[0]
		return []any{int64(0), int64(3), int64(1500), int64(3500)}, nil
	})

	decision, err := limiter.Allow(context.Background(), "203.0.113.10", Policy{
		Name:           "public-read",
		Capacity:       5,
		RefillTokens:   1,
		RefillInterval: time.Second,
	})
	if err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if decision.Allowed || decision.Limit != 5 || decision.Remaining != 3 || decision.RetryAfter != 1500*time.Millisecond || decision.ResetAfter != 3500*time.Millisecond {
		t.Fatalf("decision = %#v, want denied decision from script", decision)
	}
	if strings.Contains(capturedKey, "203.0.113.10") || !strings.HasPrefix(capturedKey, "heyblog:ratelimit:v1:public-read:") {
		t.Fatalf("Redis key = %q, want namespaced hash without raw identifier", capturedKey)
	}
}

func TestAllowBoundsVeryLargeBucketTTL(t *testing.T) {
	t.Parallel()

	var ttlMilliseconds int64
	limiter := newWithEval(func(_ context.Context, _ []string, arguments ...any) ([]any, error) {
		ttlMilliseconds = arguments[2].(int64)
		return []any{int64(1), int64(0), int64(0), int64(0)}, nil
	})
	_, err := limiter.Allow(context.Background(), "client", Policy{
		Name:           "large",
		Capacity:       1 << 52,
		RefillTokens:   1,
		RefillInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if ttlMilliseconds != maximumTTL.Milliseconds() {
		t.Fatalf("TTL = %dms, want bounded %dms", ttlMilliseconds, maximumTTL.Milliseconds())
	}
}

func TestAllowReturnsRedisFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("redis unavailable")
	limiter := newWithEval(func(context.Context, []string, ...any) ([]any, error) {
		return nil, wantErr
	})
	_, err := limiter.Allow(context.Background(), "client", Policy{Name: "test", Capacity: 1, RefillTokens: 1, RefillInterval: time.Second})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Allow() error = %v, want wrapped Redis error", err)
	}
}

func TestAllowRejectsInvalidPolicyBeforeRedis(t *testing.T) {
	t.Parallel()

	called := false
	limiter := newWithEval(func(context.Context, []string, ...any) ([]any, error) {
		called = true
		return nil, nil
	})
	_, err := limiter.Allow(context.Background(), "client", Policy{Name: "Bad Policy", Capacity: 0})
	if err == nil {
		t.Fatal("Allow() error = nil, want invalid policy error")
	}
	if called {
		t.Fatal("Redis was called for an invalid policy")
	}
}
