package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"heyblog-api/internal/ratelimit"
)

type recordingRateLimiter struct {
	calls     []rateLimitCall
	decisions []ratelimit.Decision
	err       error
}

type rateLimitCall struct {
	identifier string
	policy     ratelimit.Policy
}

func (limiter *recordingRateLimiter) Allow(_ context.Context, identifier string, policy ratelimit.Policy) (ratelimit.Decision, error) {
	limiter.calls = append(limiter.calls, rateLimitCall{identifier: identifier, policy: policy})
	if limiter.err != nil {
		return ratelimit.Decision{}, limiter.err
	}
	decision := ratelimit.Decision{Allowed: true, Limit: policy.Capacity, Remaining: policy.Capacity - 1}
	if len(limiter.decisions) >= len(limiter.calls) {
		decision = limiter.decisions[len(limiter.calls)-1]
	}
	return decision, nil
}

func TestAllowMailRequestAppliesSharedEmailAndGlobalPolicies(t *testing.T) {
	t.Parallel()

	limiter := &recordingRateLimiter{}
	decision, err := allowMailRequest(context.Background(), limiter, " Reader@Example.Test ")
	if err != nil {
		t.Fatalf("allowMailRequest() error = %v", err)
	}
	if !decision.Allowed || len(limiter.calls) != 3 {
		t.Fatalf("decision = %#v, calls = %#v; want three allowed policies", decision, limiter.calls)
	}
	want := []rateLimitCall{
		{identifier: "reader@example.test", policy: ratelimit.Policy{Name: "auth-mail-email-cooldown", Capacity: 1, RefillTokens: 1, RefillInterval: time.Minute}},
		{identifier: "reader@example.test", policy: ratelimit.Policy{Name: "auth-mail-email-hourly", Capacity: 10, RefillTokens: 10, RefillInterval: time.Hour}},
		{identifier: "global", policy: ratelimit.Policy{Name: "auth-mail-global-hourly", Capacity: 500, RefillTokens: 500, RefillInterval: time.Hour}},
	}
	if len(limiter.calls) != len(want) {
		t.Fatalf("calls = %#v, want %#v", limiter.calls, want)
	}
	for index := range want {
		if limiter.calls[index] != want[index] {
			t.Fatalf("call %d = %#v, want %#v", index, limiter.calls[index], want[index])
		}
	}
}

func TestAllowMailRequestStopsAtFirstDeniedPolicy(t *testing.T) {
	t.Parallel()

	denied := ratelimit.Decision{Allowed: false, Limit: 1, RetryAfter: time.Minute}
	limiter := &recordingRateLimiter{decisions: []ratelimit.Decision{denied}}
	decision, err := allowMailRequest(context.Background(), limiter, "reader@example.test")
	if err != nil {
		t.Fatalf("allowMailRequest() error = %v", err)
	}
	if decision != denied || len(limiter.calls) != 1 {
		t.Fatalf("decision = %#v, calls = %d; want first denial", decision, len(limiter.calls))
	}
}

func TestAllowMailRequestReturnsRedisFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("redis unavailable")
	_, err := allowMailRequest(context.Background(), &recordingRateLimiter{err: wantErr}, "reader@example.test")
	if !errors.Is(err, wantErr) {
		t.Fatalf("allowMailRequest() error = %v, want Redis failure", err)
	}
}
