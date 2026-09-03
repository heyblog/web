package auth

import (
	"context"
	"time"

	"heyblog-api/internal/httpapi"
	"heyblog-api/internal/ratelimit"
)

type mailRateLimiter interface {
	Allow(context.Context, string, ratelimit.Policy) (ratelimit.Decision, error)
}

type mailRateLimit struct {
	identifier string
	policy     ratelimit.Policy
}

func mailRateLimits(email string) []mailRateLimit {
	normalized := normalizeEmail(email)
	return []mailRateLimit{
		{identifier: normalized, policy: ratelimit.Policy{Name: "auth-mail-email-cooldown", Capacity: 1, RefillTokens: 1, RefillInterval: time.Minute}},
		{identifier: normalized, policy: ratelimit.Policy{Name: "auth-mail-email-hourly", Capacity: 10, RefillTokens: 10, RefillInterval: time.Hour}},
		{identifier: "global", policy: ratelimit.Policy{Name: "auth-mail-global-hourly", Capacity: 500, RefillTokens: 500, RefillInterval: time.Hour}},
	}
}

func allowMailRequest(ctx context.Context, limiter mailRateLimiter, email string) (ratelimit.Decision, error) {
	var decision ratelimit.Decision
	for _, limit := range mailRateLimits(email) {
		var err error
		decision, err = limiter.Allow(ctx, limit.identifier, limit.policy)
		if err != nil || !decision.Allowed {
			return decision, err
		}
	}
	return decision, nil
}

func enforceMailRequest(ctx *httpapi.Context, limiter mailRateLimiter, email string) error {
	decision, err := allowMailRequest(ctx.Request.Context(), limiter, email)
	return httpapi.EnforceRateLimitDecision(ctx, decision, err)
}
