package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"strings"

	"heyblog-api/internal/apperror"
)

// BearerAuthorization authenticates a fixed internal-service token without
// retaining or comparing the secret in variable time.
func BearerAuthorization(expectedToken, realm string) Middleware {
	expectedDigest := sha256.Sum256([]byte(expectedToken))
	challenge := fmt.Sprintf("Bearer realm=%q", realm)
	return func(next Endpoint) Endpoint {
		return func(ctx *Context) (Response, error) {
			scheme, token, found := strings.Cut(ctx.Request.Header.Get("Authorization"), " ")
			actualDigest := sha256.Sum256([]byte(token))
			if !found || !strings.EqualFold(scheme, "Bearer") ||
				subtle.ConstantTimeCompare(actualDigest[:], expectedDigest[:]) != 1 {
				ctx.Header("WWW-Authenticate", challenge)
				return Response{}, apperror.New(
					apperror.KindUnauthorized,
					apperror.CodeUnauthorized,
					"authentication is required",
				)
			}
			return next(ctx)
		}
	}
}
