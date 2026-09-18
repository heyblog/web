package auth

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"heyblog-api/internal/httpapi"
)

func registerSessionRoutes(api huma.API, service *Service, limiter mailRateLimiter, build authOperationBuilder) {
	authErrors := []int{
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusConflict,
		http.StatusUnprocessableEntity,
		http.StatusTooManyRequests,
		http.StatusServiceUnavailable,
	}
	operation := func(id, method, path, summary string) huma.Operation {
		return build(huma.Operation{
			OperationID: id,
			Method:      method,
			Path:        path,
			Summary:     summary,
			Tags:        []string{"authentication"},
			Errors:      authErrors,
		})
	}

	registerOperation := operation("register-user", http.MethodPost, "/auth/register", "Register a user")
	registerOperation.DefaultStatus = http.StatusAccepted
	httpapi.Register(api, registerOperation, func(ctx context.Context, input *bodyInput[registerRequest]) (*acceptedOutput, error) {
		if err := enforceMailRequest(ctx, limiter, input.Body.Email); err != nil {
			return nil, err
		}
		if err := service.Register(ctx, input.Body.Username, input.Body.Email, input.Body.Password); err != nil {
			return nil, mapError(err)
		}
		return &acceptedOutput{Body: acceptedResponseBody{Status: "verification_required"}}, nil
	})

	httpapi.Register(api, operation("login-user", http.MethodPost, "/auth/login", "Log in"), func(ctx context.Context, input *bodyInput[loginRequest]) (*userOutput, error) {
		user, cookies, err := service.Login(ctx, input.Body.Identifier, input.Body.Password)
		if err != nil {
			return nil, mapError(err)
		}
		return &userOutput{
			CacheControl: "no-store",
			SetCookie:    cookieValues(cookies, service.config),
			Body:         userResponseBody{User: user},
		}, nil
	})

	currentOperation := operation("get-current-user", http.MethodGet, "/auth/me", "Get the current user")
	currentOperation.Security = []map[string][]string{{"webToken": {}, "accessCookie": {}}}
	httpapi.Register(api, currentOperation, func(ctx context.Context, _ *emptyInput) (*userOutput, error) {
		user, err := service.Current(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, mapError(err)
		}
		return &userOutput{Body: userResponseBody{User: user}}, nil
	})

	refreshOperation := operation("refresh-session", http.MethodPost, "/auth/refresh", "Refresh the session")
	refreshOperation.Security = []map[string][]string{{"webToken": {}, "refreshCookie": {}}}
	httpapi.Register(api, refreshOperation, func(ctx context.Context, _ *emptyInput) (*userOutput, error) {
		user, cookies, err := service.Refresh(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, mapError(err)
		}
		return &userOutput{
			CacheControl: "no-store",
			SetCookie:    cookieValues(cookies, service.config),
			Body:         userResponseBody{User: user},
		}, nil
	})

	logoutOperation := operation("logout-user", http.MethodPost, "/auth/logout", "Log out")
	logoutOperation.DefaultStatus = http.StatusNoContent
	httpapi.Register(api, logoutOperation, func(ctx context.Context, _ *emptyInput) (*noContentOutput, error) {
		if err := service.Logout(ctx, httpapi.Request(ctx)); err != nil {
			return nil, mapError(err)
		}
		return &noContentOutput{
			Status:       http.StatusNoContent,
			CacheControl: "no-store",
			SetCookie:    clearedCookieValues(service.config),
		}, nil
	})

	registerNoContentBodyRoute(api, service, limiter, operation)
}

func registerNoContentBodyRoute(
	api huma.API,
	service *Service,
	limiter mailRateLimiter,
	operation func(string, string, string, string) huma.Operation,
) {
	type noContentHandler[T any] func(context.Context, T) error
	register := func(id, path, summary string, handler noContentHandler[emailRequest]) {
		op := operation(id, http.MethodPost, path, summary)
		op.DefaultStatus = http.StatusNoContent
		httpapi.Register(api, op, func(ctx context.Context, input *bodyInput[emailRequest]) (*noContentOutput, error) {
			if err := handler(ctx, input.Body); err != nil {
				return nil, err
			}
			return &noContentOutput{Status: http.StatusNoContent}, nil
		})
	}

	verifyOperation := operation("verify-email", http.MethodPost, "/auth/verify-email", "Verify an email address")
	verifyOperation.DefaultStatus = http.StatusNoContent
	httpapi.Register(api, verifyOperation, func(ctx context.Context, input *bodyInput[verifyRequest]) (*noContentOutput, error) {
		if err := service.VerifyEmail(ctx, input.Body.Email, input.Body.Code); err != nil {
			return nil, mapError(err)
		}
		return &noContentOutput{Status: http.StatusNoContent}, nil
	})

	register("resend-email-verification", "/auth/verify-email/resend", "Resend email verification", func(ctx context.Context, input emailRequest) error {
		if err := enforceMailRequest(ctx, limiter, input.Email); err != nil {
			return err
		}
		return mapOptionalError(service.ResendVerification(ctx, input.Email))
	})
	register("request-password-reset", "/auth/password/forgot", "Request a password reset", func(ctx context.Context, input emailRequest) error {
		if err := enforceMailRequest(ctx, limiter, input.Email); err != nil {
			return err
		}
		return mapOptionalError(service.ForgotPassword(ctx, input.Email))
	})

	resetOperation := operation("reset-password", http.MethodPost, "/auth/password/reset", "Reset a password")
	resetOperation.DefaultStatus = http.StatusNoContent
	httpapi.Register(api, resetOperation, func(ctx context.Context, input *bodyInput[resetRequest]) (*noContentOutput, error) {
		if err := service.ResetPassword(ctx, input.Body.Token, input.Body.Password); err != nil {
			return nil, mapError(err)
		}
		return &noContentOutput{Status: http.StatusNoContent}, nil
	})

	setPasswordOperation := operation("set-password", http.MethodPost, "/auth/password", "Set or change the password")
	setPasswordOperation.Security = []map[string][]string{{"webToken": {}, "accessCookie": {}}}
	httpapi.Register(api, setPasswordOperation, func(ctx context.Context, input *bodyInput[setPasswordRequest]) (*userOutput, error) {
		user, cookies, err := service.SetPassword(ctx, httpapi.Request(ctx), input.Body.CurrentPassword, input.Body.NextPassword)
		if err != nil {
			return nil, mapError(err)
		}
		return &userOutput{
			CacheControl: "no-store",
			SetCookie:    cookieValues(cookies, service.config),
			Body:         userResponseBody{User: user},
		}, nil
	})
}

func mapOptionalError(err error) error {
	if err == nil {
		return nil
	}
	return mapError(err)
}
