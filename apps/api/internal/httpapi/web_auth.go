package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"

	"heyblog-api/internal/apperror"
)

const WebTokenHeader = "X-HeyBlog-Web-Token" // #nosec G101 -- this is a header name, not a credential.

func webAuthorization(expectedToken string) Middleware {
	expectedDigest := sha256.Sum256([]byte(expectedToken))
	return func(next Endpoint) Endpoint {
		return func(ctx *Context) (Response, error) {
			actualDigest := sha256.Sum256([]byte(ctx.Request.Header.Get(WebTokenHeader)))
			if subtle.ConstantTimeCompare(actualDigest[:], expectedDigest[:]) != 1 {
				return Response{}, apperror.New(
					apperror.KindUnauthorized,
					apperror.CodeUnauthorized,
					"web service authentication is required",
				)
			}
			return next(ctx)
		}
	}
}
