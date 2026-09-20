package apikey

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"heyblog-api/internal/apperror"
	"heyblog-api/internal/httpapi"
)

type Authenticator interface {
	Authenticate(context.Context, string, AccessPolicy) (Principal, error)
}

func HumaAuthorization(service Authenticator, policy AccessPolicy) func(huma.Context, func(huma.Context)) {
	challenge := fmt.Sprintf(`Bearer realm="heyblog-api", scope="%s"`, policy.Scope)
	return func(ctx huma.Context, next func(huma.Context)) {
		scheme, token, found := strings.Cut(ctx.Header("Authorization"), " ")
		if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
			ctx.SetHeader("WWW-Authenticate", challenge)
			httpapi.RejectHumaRequest(ctx, invalidKeyError())
			return
		}
		if _, err := service.Authenticate(ctx.Context(), token, policy); err != nil {
			if errors.Is(err, ErrInsufficientScope) {
				httpapi.RejectHumaRequest(ctx, apperror.New(apperror.KindForbidden, "insufficient_scope", "the API key does not grant access to this operation"))
				return
			}
			ctx.SetHeader("WWW-Authenticate", challenge+`, error="invalid_token"`)
			httpapi.RejectHumaRequest(ctx, invalidKeyError())
			return
		}
		next(ctx)
	}
}

func HTTPError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidAudience), errors.Is(err, ErrInvalidExpiration),
		errors.Is(err, ErrInvalidName), errors.Is(err, ErrInvalidOverlap), errors.Is(err, ErrInvalidScope):
		return apperror.New(apperror.KindValidation, apperror.CodeValidationFailed, "the API client configuration is invalid")
	case errors.Is(err, ErrNotFound):
		return apperror.New(apperror.KindNotFound, apperror.CodeNotFound, "the API client or key was not found")
	case errors.Is(err, ErrClientDisabled):
		return apperror.New(apperror.KindConflict, "api_client_disabled", "the API client is disabled")
	case errors.Is(err, ErrClientHasActiveKey):
		return apperror.New(apperror.KindConflict, "api_client_has_active_key", "the API client already has an active key; rotate it instead")
	case errors.Is(err, ErrClientNameConflict):
		return apperror.New(apperror.KindConflict, "api_client_name_conflict", "the API client name is already in use")
	default:
		return apperror.Wrap(err, apperror.KindInternal, apperror.CodeInternal, "the API credential service is unavailable", "manage API credentials")
	}
}

func invalidKeyError() error {
	return apperror.New(apperror.KindUnauthorized, "invalid_api_key", "a valid API key is required")
}
