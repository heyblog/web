package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"heyblog-api/internal/platform/httpapi"
	"heyblog-api/internal/platform/ratelimit"
)

const emailChangePath = "/api/web/account/email-change"

type emailChangeHeader struct {
	RequestID string `header:"X-HeyBlog-Email-Change-ID" required:"true" maxLength:"36"`
}

type emailChangeBody[T any] struct {
	RequestID string `header:"X-HeyBlog-Email-Change-ID" required:"true" maxLength:"36"`
	Body      T
}

type emailChangeOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         EmailChangeStatus
}

type emailChangeAddressInput struct {
	Email string `json:"email" format:"email" maxLength:"320"`
}

type emailChangeSideInput struct {
	Side EmailChangeSide `json:"side" enum:"old,new"`
}

type emailChangeCodeInput struct {
	Side EmailChangeSide `json:"side" enum:"old,new"`
	Code string          `json:"code" pattern:"^[0-9]{6}$"`
}

type emailChangePasswordInput struct {
	Password string `json:"password" maxLength:"128"`
}

func registerEmailChangeRoutes(api huma.API, service *Service, limiter mailRateLimiter, build authOperationBuilder) {
	operation := func(id, method, suffix, summary string) huma.Operation {
		return build(huma.Operation{
			OperationID: id, Method: method, Path: emailChangePath + suffix, Summary: summary,
			Tags: []string{"account profile"}, Security: []map[string][]string{{"webToken": {}, "accessCookie": {}}},
			Errors: []int{400, 401, 403, 404, 409, 422, 429, 503}, MaxBodyBytes: 4096,
		})
	}
	httpapi.Register(api, operation("get-email-change", http.MethodGet, "", "Read email change status"), func(ctx context.Context, _ *emptyInput) (*emailChangeOutput, error) {
		return emailChangeResponse(service.EmailChange(ctx, httpapi.Request(ctx)))
	})
	httpapi.Register(api, operation("start-email-change", http.MethodPost, "", "Start an email change"), func(ctx context.Context, input *bodyInput[emailChangeAddressInput]) (*emailChangeOutput, error) {
		return emailChangeResponse(service.StartEmailChange(ctx, httpapi.Request(ctx), input.Body.Email))
	})
	httpapi.Register(api, operation("cancel-email-change", http.MethodDelete, "", "Cancel the email change"), func(ctx context.Context, input *emailChangeHeader) (*emailChangeOutput, error) {
		return emailChangeResponse(service.CancelEmailChange(ctx, httpapi.Request(ctx), input.RequestID))
	})
	httpapi.Register(api, operation("send-email-change-code", http.MethodPost, "/send-code", "Send an email change code"), func(ctx context.Context, input *emailChangeBody[emailChangeSideInput]) (*emailChangeOutput, error) {
		return emailChangeResponse(service.SendEmailChangeCode(ctx, httpapi.Request(ctx), input.RequestID, input.Body.Side, func(account, email string) error {
			policy := ratelimit.Policy{Name: "auth-email-change-mail-account", Capacity: 6, RefillTokens: 6, RefillInterval: time.Hour}
			if err := enforceEmailChangeRate(ctx, limiter, account, policy); err != nil {
				return err
			}
			return enforceMailRequest(ctx, limiter, email)
		}))
	})
	httpapi.Register(api, operation("verify-email-change-code", http.MethodPost, "/verify-code", "Verify an email change code"), func(ctx context.Context, input *emailChangeBody[emailChangeCodeInput]) (*emailChangeOutput, error) {
		return emailChangeResponse(service.VerifyEmailChangeCode(ctx, httpapi.Request(ctx), input.RequestID, input.Body.Side, input.Body.Code))
	})
	httpapi.Register(api, operation("verify-email-change-password", http.MethodPost, "/verify-password", "Confirm identity with the current password"), func(ctx context.Context, input *emailChangeBody[emailChangePasswordInput]) (*emailChangeOutput, error) {
		policy := ratelimit.Policy{Name: "auth-email-change-password-account", Capacity: 5, RefillTokens: 5, RefillInterval: 10 * time.Minute}
		return emailChangeResponse(service.VerifyEmailChangePassword(ctx, httpapi.Request(ctx), input.RequestID, input.Body.Password, func(account string) error {
			return enforceEmailChangeRate(ctx, limiter, account, policy)
		}))
	})
	httpapi.Register(api, operation("complete-email-change", http.MethodPost, "/complete", "Complete the verified email change"), func(ctx context.Context, input *emailChangeBody[emptyInput]) (*userOutput, error) {
		user, cookies, err := service.CompleteEmailChange(ctx, httpapi.Request(ctx), input.RequestID)
		if err != nil {
			return nil, mapError(err)
		}
		return &userOutput{CacheControl: "private, no-store", SetCookie: cookieValues(cookies, service.config), Body: userResponseBody{User: user}}, nil
	})
}

func enforceEmailChangeRate(ctx context.Context, limiter mailRateLimiter, account string, policy ratelimit.Policy) error {
	decision, err := limiter.Allow(ctx, account, policy)
	return httpapi.EnforceRateLimitDecisionForRequest(ctx, decision, err)
}

func emailChangeResponse(status EmailChangeStatus, err error) (*emailChangeOutput, error) {
	if err != nil {
		return nil, mapError(err)
	}
	return &emailChangeOutput{CacheControl: "private, no-store", Body: status}, nil
}
