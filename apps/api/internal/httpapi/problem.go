package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"runtime/debug"
	"sort"
	"time"

	"github.com/gin-gonic/gin"

	"heyblog-api/internal/apperror"
)

const ProblemMediaType = "application/problem+json"

type problem struct {
	Type          string                  `json:"type"`
	Title         string                  `json:"title"`
	Status        int                     `json:"status"`
	Detail        string                  `json:"detail"`
	Instance      string                  `json:"instance"`
	Code          string                  `json:"code"`
	RequestID     string                  `json:"request_id"`
	InvalidParams []apperror.InvalidParam `json:"invalid_params,omitempty"`
}

type problemDescriptor struct {
	status int
	title  string
	detail string
	code   string
	params []apperror.InvalidParam
	op     string
}

func errorBoundary(logger *slog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		started := time.Now()
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.ErrorContext(ctx.Request.Context(), "request panic",
					"event", "request_panic",
					"request_id", RequestID(ctx),
					"panic_type", fmt.Sprintf("%T", recovered),
					"stack", string(debug.Stack()),
				)
				if !ctx.Writer.Written() {
					writeProblem(ctx, internalProblem())
				}
				ctx.Abort()
			}
			logAccess(ctx, logger, started)
		}()
		ctx.Next()
		if len(ctx.Errors) == 0 {
			return
		}
		err := ctx.Errors.Last().Err
		descriptor := describeError(err)
		logRequestError(ctx, logger, descriptor, err)
		if ctx.Writer.Written() {
			return
		}
		writeProblem(ctx, descriptor)
	}
}

func describeError(err error) problemDescriptor {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return problemDescriptor{
			status: http.StatusRequestEntityTooLarge,
			title:  "Request Entity Too Large",
			detail: "request body exceeds the configured limit",
			code:   apperror.CodeRequestTooLarge,
		}
	}

	var applicationError *apperror.Error
	if !errors.As(err, &applicationError) {
		return problemDescriptor{
			status: http.StatusInternalServerError,
			title:  "Internal Server Error",
			detail: "an unexpected error occurred",
			code:   apperror.CodeInternal,
		}
	}

	status, title, fallbackDetail, fallbackCode := kindHTTP(applicationError.Kind())
	detail := applicationError.PublicDetail()
	if detail == "" {
		detail = fallbackDetail
	}
	code := applicationError.Code()
	if code == "" {
		code = fallbackCode
	}
	return problemDescriptor{
		status: status,
		title:  title,
		detail: detail,
		code:   code,
		params: applicationError.InvalidParams(),
		op:     applicationError.Operation(),
	}
}

func kindHTTP(kind apperror.Kind) (int, string, string, string) {
	switch kind {
	case apperror.KindBadRequest:
		return http.StatusBadRequest, "Bad Request", "the request is malformed", apperror.CodeBadRequest
	case apperror.KindValidation:
		return http.StatusUnprocessableEntity, "Unprocessable Entity", "request validation failed", apperror.CodeValidationFailed
	case apperror.KindUnauthorized:
		return http.StatusUnauthorized, "Unauthorized", "authentication is required", apperror.CodeUnauthorized
	case apperror.KindForbidden:
		return http.StatusForbidden, "Forbidden", "the request is not allowed", apperror.CodeForbidden
	case apperror.KindNotFound:
		return http.StatusNotFound, "Not Found", "the requested resource was not found", apperror.CodeNotFound
	case apperror.KindMethodNotAllowed:
		return http.StatusMethodNotAllowed, "Method Not Allowed", "the method is not allowed for this resource", apperror.CodeMethodNotAllowed
	case apperror.KindConflict:
		return http.StatusConflict, "Conflict", "the request conflicts with current state", apperror.CodeConflict
	case apperror.KindTooLarge:
		return http.StatusRequestEntityTooLarge, "Request Entity Too Large", "request body exceeds the configured limit", apperror.CodeRequestTooLarge
	case apperror.KindRateLimited:
		return http.StatusTooManyRequests, "Too Many Requests", "request rate limit exceeded", apperror.CodeRateLimited
	case apperror.KindUnavailable:
		return http.StatusServiceUnavailable, "Service Unavailable", "service is temporarily unavailable", apperror.CodeServiceUnavailable
	case apperror.KindInternal:
		return http.StatusInternalServerError, "Internal Server Error", "an unexpected error occurred", apperror.CodeInternal
	default:
		return http.StatusInternalServerError, "Internal Server Error", "an unexpected error occurred", apperror.CodeInternal
	}
}

func writeProblem(ctx *gin.Context, descriptor problemDescriptor) {
	payload := problem{
		Type:          "urn:heyblog:problem:" + descriptor.code,
		Title:         descriptor.title,
		Status:        descriptor.status,
		Detail:        descriptor.detail,
		Instance:      ctx.Request.URL.EscapedPath(),
		Code:          descriptor.code,
		RequestID:     RequestID(ctx),
		InvalidParams: descriptor.params,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		body = []byte(`{"type":"urn:heyblog:problem:internal_error","title":"Internal Server Error","status":500,"detail":"an unexpected error occurred","code":"internal_error"}`)
		descriptor.status = http.StatusInternalServerError
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.Data(descriptor.status, ProblemMediaType, body)
}

func logRequestError(ctx *gin.Context, logger *slog.Logger, descriptor problemDescriptor, err error) {
	level := slog.LevelWarn
	if descriptor.status >= http.StatusInternalServerError {
		level = slog.LevelError
	}
	attributes := []slog.Attr{
		slog.String("event", "request_error"),
		slog.String("request_id", RequestID(ctx)),
		slog.String("method", ctx.Request.Method),
		slog.String("path", ctx.Request.URL.Path),
		slog.Int("status", descriptor.status),
		slog.String("code", descriptor.code),
		slog.String("error_type", reflect.TypeOf(err).String()),
	}
	if descriptor.op != "" {
		attributes = append(attributes, slog.String("operation", descriptor.op))
	}
	components := dependencyComponents(err)
	if len(components) == 1 {
		attributes = append(attributes, slog.String("dependency", components[0]))
	} else if len(components) > 1 {
		attributes = append(attributes, slog.Any("dependencies", components))
	}
	logger.LogAttrs(ctx.Request.Context(), level, "request failed", attributes...)
}

func dependencyComponents(err error) []string {
	seen := make(map[string]struct{})
	var visit func(error)
	visit = func(current error) {
		if current == nil {
			return
		}
		if component, ok := current.(interface{ Component() string }); ok {
			name := component.Component()
			if name != "" {
				seen[name] = struct{}{}
			}
		}
		// Walk the exact unwrap tree so every joined dependency failure is retained;
		// errors.As intentionally stops at the first match.
		switch wrapped := current.(type) { //nolint:errorlint
		case interface{ Unwrap() []error }:
			for _, child := range wrapped.Unwrap() {
				visit(child)
			}
		case interface{ Unwrap() error }:
			visit(wrapped.Unwrap())
		}
	}
	visit(err)

	components := make([]string, 0, len(seen))
	for component := range seen {
		components = append(components, component)
	}
	sort.Strings(components)
	return components
}

func internalProblem() problemDescriptor {
	return problemDescriptor{
		status: http.StatusInternalServerError,
		title:  "Internal Server Error",
		detail: "an unexpected error occurred",
		code:   apperror.CodeInternal,
	}
}
