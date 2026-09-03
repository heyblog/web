package httpapi

import (
	"context"
	"strconv"
	"time"

	"heyblog-api/internal/apperror"
	"heyblog-api/internal/ratelimit"
)

type RateLimiter interface {
	Allow(context.Context, string, ratelimit.Policy) (ratelimit.Decision, error)
}

func RateLimit(limiter RateLimiter, policy ratelimit.Policy) Middleware {
	return func(next Endpoint) Endpoint {
		return func(ctx *Context) (Response, error) {
			if err := EnforceRateLimit(ctx, limiter, ctx.ClientIP(), policy); err != nil {
				return Response{}, err
			}
			return next(ctx)
		}
	}
}

func EnforceRateLimit(ctx *Context, limiter RateLimiter, identifier string, policy ratelimit.Policy) error {
	decision, err := limiter.Allow(ctx.Request.Context(), identifier, policy)
	return EnforceRateLimitDecision(ctx, decision, err)
}

func EnforceRateLimitDecision(ctx *Context, decision ratelimit.Decision, err error) error {
	if err != nil {
		return apperror.Wrap(
			err,
			apperror.KindUnavailable,
			apperror.CodeServiceUnavailable,
			"request rate limit is temporarily unavailable",
			"apply request rate limit",
		)
	}
	setRateLimitHeaders(ctx, decision)
	if decision.Allowed {
		return nil
	}
	ctx.Header("Retry-After", durationSeconds(decision.RetryAfter))
	return apperror.New(
		apperror.KindRateLimited,
		apperror.CodeRateLimited,
		"request rate limit exceeded",
	)
}

func setRateLimitHeaders(ctx *Context, decision ratelimit.Decision) {
	ctx.Header("RateLimit-Limit", strconv.FormatInt(decision.Limit, 10))
	ctx.Header("RateLimit-Remaining", strconv.FormatInt(decision.Remaining, 10))
	ctx.Header("RateLimit-Reset", durationSeconds(decision.ResetAfter))
}

func durationSeconds(duration time.Duration) string {
	if duration <= 0 {
		return "0"
	}
	seconds := (duration + time.Second - 1) / time.Second
	return strconv.FormatInt(int64(seconds), 10)
}
