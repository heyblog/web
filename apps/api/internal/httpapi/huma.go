package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"

	"heyblog-api/internal/apperror"
	"heyblog-api/internal/ratelimit"
)

type requestContextKey struct{}

type requestContext struct {
	gin    *gin.Context
	logger *slog.Logger
}

func HumaWebAuthorization(expectedToken string) func(huma.Context, func(huma.Context)) {
	expectedDigest := sha256.Sum256([]byte(expectedToken))
	return func(ctx huma.Context, next func(huma.Context)) {
		actualDigest := sha256.Sum256([]byte(ctx.Header(WebTokenHeader)))
		if subtle.ConstantTimeCompare(actualDigest[:], expectedDigest[:]) != 1 {
			rejectHumaRequest(ctx, apperror.New(
				apperror.KindUnauthorized,
				apperror.CodeUnauthorized,
				"web service authentication is required",
			))
			return
		}
		next(ctx)
	}
}

func HumaBearerAuthorization(expectedToken, realm string) func(huma.Context, func(huma.Context)) {
	expectedDigest := sha256.Sum256([]byte(expectedToken))
	challenge := fmt.Sprintf("Bearer realm=%q", realm)
	return func(ctx huma.Context, next func(huma.Context)) {
		scheme, token, found := strings.Cut(ctx.Header("Authorization"), " ")
		actualDigest := sha256.Sum256([]byte(token))
		if !found || !strings.EqualFold(scheme, "Bearer") ||
			subtle.ConstantTimeCompare(actualDigest[:], expectedDigest[:]) != 1 {
			ctx.SetHeader("WWW-Authenticate", challenge)
			rejectHumaRequest(ctx, apperror.New(
				apperror.KindUnauthorized,
				apperror.CodeUnauthorized,
				"authentication is required",
			))
			return
		}
		next(ctx)
	}
}

func HumaRateLimit(limiter RateLimiter, policy ratelimit.Policy) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		native := humagin.Unwrap(ctx)
		decision, err := limiter.Allow(ctx.Context(), native.ClientIP(), policy)
		if err != nil {
			rejectHumaRequest(ctx, apperror.Wrap(
				err,
				apperror.KindUnavailable,
				apperror.CodeServiceUnavailable,
				"request rate limit is temporarily unavailable",
				"apply request rate limit",
			))
			return
		}
		ctx.SetHeader("RateLimit-Limit", fmt.Sprintf("%d", decision.Limit))
		ctx.SetHeader("RateLimit-Remaining", fmt.Sprintf("%d", decision.Remaining))
		ctx.SetHeader("RateLimit-Reset", durationSeconds(decision.ResetAfter))
		if !decision.Allowed {
			ctx.SetHeader("Retry-After", durationSeconds(decision.RetryAfter))
			rejectHumaRequest(ctx, apperror.New(
				apperror.KindRateLimited,
				apperror.CodeRateLimited,
				"request rate limit exceeded",
			))
			return
		}
		next(ctx)
	}
}

func rejectHumaRequest(ctx huma.Context, err error) {
	native := humagin.Unwrap(ctx)
	_ = native.Error(err)
	native.Abort()
}

func SetDeadlines(ctx context.Context, deadline time.Time) error {
	native := NativeContext(ctx)
	if native == nil {
		return errors.New("native HTTP context is unavailable")
	}
	controller := http.NewResponseController(native.Writer)
	if err := controller.SetReadDeadline(deadline); err != nil {
		return err
	}
	return controller.SetWriteDeadline(deadline)
}

func init() {
	huma.NewError = func(status int, message string, errs ...error) huma.StatusError {
		return newHumaProblem(nil, status, message, errs...)
	}
	huma.NewErrorWithContext = func(ctx huma.Context, status int, message string, errs ...error) huma.StatusError {
		return newHumaProblem(ctx, status, message, errs...)
	}
}

func attachRequestContext(logger *slog.Logger) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		native := humagin.Unwrap(ctx)
		next(huma.WithValue(ctx, requestContextKey{}, requestContext{gin: native, logger: logger}))
	}
}

func Register[I, O any](
	api huma.API,
	operation huma.Operation,
	handler func(context.Context, *I) (*O, error),
) {
	huma.Register(api, operation, func(ctx context.Context, input *I) (*O, error) {
		output, err := handler(ctx, input)
		if err != nil {
			return nil, HTTPError(ctx, err)
		}
		return output, nil
	})
}

func Request(ctx context.Context) *http.Request {
	request, _ := ctx.Value(requestContextKey{}).(requestContext)
	if request.gin == nil {
		return nil
	}
	return request.gin.Request
}

func NativeContext(ctx context.Context) *gin.Context {
	request, _ := ctx.Value(requestContextKey{}).(requestContext)
	return request.gin
}

func HTTPError(ctx context.Context, err error) error {
	descriptor := describeError(err)
	request, _ := ctx.Value(requestContextKey{}).(requestContext)
	if request.gin != nil && request.logger != nil {
		logRequestError(request.gin, request.logger, descriptor, err)
	}
	return problemFromDescriptor(request.gin, descriptor)
}

func newHumaProblem(ctx huma.Context, status int, message string, errs ...error) *Problem {
	descriptor := descriptorForHumaError(status, message, errs)
	var native *gin.Context
	if ctx != nil {
		native = humagin.Unwrap(ctx)
		request, _ := ctx.Context().Value(requestContextKey{}).(requestContext)
		if request.logger != nil && len(errs) > 0 {
			logRequestError(native, request.logger, descriptor, errors.Join(errs...))
		}
	}
	return problemFromDescriptor(native, descriptor)
}

func descriptorForHumaError(status int, message string, errs []error) problemDescriptor {
	for _, err := range errs {
		var applicationError *apperror.Error
		if errors.As(err, &applicationError) {
			return describeError(err)
		}
		var detailer huma.ErrorDetailer
		if errors.As(err, &detailer) {
			detail := detailer.ErrorDetail()
			switch {
			case strings.Contains(detail.Message, "request body too large"):
				status = http.StatusRequestEntityTooLarge
			case strings.HasPrefix(detail.Message, "cannot read multipart form:"):
				status = http.StatusBadRequest
			}
		}
	}
	if status == 0 {
		status = http.StatusInternalServerError
	}
	kind := apperror.KindInternal
	code := apperror.CodeInternal
	detail := "an unexpected error occurred"
	switch status {
	case http.StatusBadRequest:
		kind, code, detail = apperror.KindBadRequest, apperror.CodeBadRequest, "the request is malformed"
	case http.StatusUnauthorized:
		kind, code, detail = apperror.KindUnauthorized, apperror.CodeUnauthorized, "authentication is required"
	case http.StatusForbidden:
		kind, code, detail = apperror.KindForbidden, apperror.CodeForbidden, "the request is not allowed"
	case http.StatusNotFound:
		kind, code, detail = apperror.KindNotFound, apperror.CodeNotFound, "the requested resource was not found"
	case http.StatusMethodNotAllowed:
		kind, code, detail = apperror.KindMethodNotAllowed, apperror.CodeMethodNotAllowed, "the method is not allowed for this resource"
	case http.StatusConflict:
		kind, code, detail = apperror.KindConflict, apperror.CodeConflict, "the request conflicts with current state"
	case http.StatusRequestEntityTooLarge:
		kind, code, detail = apperror.KindTooLarge, apperror.CodeRequestTooLarge, "request body exceeds the configured limit"
	case http.StatusUnprocessableEntity:
		kind, code, detail = apperror.KindValidation, apperror.CodeValidationFailed, "request validation failed"
	case http.StatusTooManyRequests:
		kind, code, detail = apperror.KindRateLimited, apperror.CodeRateLimited, "request rate limit exceeded"
	case http.StatusServiceUnavailable:
		kind, code, detail = apperror.KindUnavailable, apperror.CodeServiceUnavailable, "service is temporarily unavailable"
	}
	if status < http.StatusInternalServerError && message != "" && message != "validation failed" {
		detail = message
	}
	params := make([]apperror.InvalidParam, 0, len(errs))
	for _, err := range errs {
		var detailer huma.ErrorDetailer
		if !errors.As(err, &detailer) {
			continue
		}
		detail := detailer.ErrorDetail()
		name := strings.TrimPrefix(detail.Location, "body.")
		params = append(params, apperror.InvalidParam{Name: name, Reason: detail.Message})
	}
	statusCode, title, _, _ := kindHTTP(kind)
	return problemDescriptor{status: statusCode, title: title, detail: detail, code: code, params: params}
}

func problemFromDescriptor(ctx *gin.Context, descriptor problemDescriptor) *Problem {
	instance := ""
	requestID := ""
	if ctx != nil {
		instance = ctx.Request.URL.EscapedPath()
		requestID = RequestID(ctx)
	}
	return &Problem{
		Type:          "urn:heyblog:problem:" + descriptor.code,
		Title:         descriptor.title,
		Status:        descriptor.status,
		Detail:        descriptor.detail,
		Instance:      instance,
		Code:          descriptor.code,
		RequestID:     requestID,
		InvalidParams: descriptor.params,
	}
}
