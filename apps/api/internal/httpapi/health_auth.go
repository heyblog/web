package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"strings"

	"heyblog-api/internal/apperror"
)

const healthAuthenticationChallenge = `Bearer realm="heyblog-health"`

func healthAuthorization(expectedToken string) Middleware {
	expectedDigest := sha256.Sum256([]byte(expectedToken))
	return func(next Endpoint) Endpoint {
		return func(ctx *Context) (Response, error) {
			scheme, token, found := strings.Cut(ctx.Request.Header.Get("Authorization"), " ")
			actualDigest := sha256.Sum256([]byte(token))
			if !found || !strings.EqualFold(scheme, "Bearer") ||
				subtle.ConstantTimeCompare(actualDigest[:], expectedDigest[:]) != 1 {
				ctx.Header("WWW-Authenticate", healthAuthenticationChallenge)
				return Response{}, apperror.New(
					apperror.KindUnauthorized,
					apperror.CodeUnauthorized,
					"health check authentication is required",
				)
			}
			return next(ctx)
		}
	}
}
