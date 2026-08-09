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
			decision, err := limiter.Allow(ctx.Request.Context(), ctx.ClientIP(), policy)
			if err != nil {
				return Response{}, apperror.Wrap(
					err,
					apperror.KindUnavailable,
					apperror.CodeServiceUnavailable,
					"request rate limit is temporarily unavailable",
					"apply request rate limit",
				)
			}
			setRateLimitHeaders(ctx, decision)
			if !decision.Allowed {
				ctx.Header("Retry-After", durationSeconds(decision.RetryAfter))
				return Response{}, apperror.New(
					apperror.KindRateLimited,
					apperror.CodeRateLimited,
					"request rate limit exceeded",
				)
			}
			return next(ctx)
		}
	}
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
