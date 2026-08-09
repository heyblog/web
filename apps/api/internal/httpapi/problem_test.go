package httpapi

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"heyblog-api/internal/apperror"
)

func TestErrorBoundaryRendersTypedProblem(t *testing.T) {
	t.Parallel()

	router, logs := testErrorRouter(t, func(*Context) (Response, error) {
		return Response{}, apperror.New(
			apperror.KindValidation,
			apperror.CodeValidationFailed,
			"request validation failed",
		).WithInvalidParams([]apperror.InvalidParam{{Name: "title", Reason: "is required"}})
	})

	request := httptest.NewRequest(http.MethodPost, "/test", nil)
	request.Header.Set(RequestIDHeader, "request-123")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnprocessableEntity)
	}
	if response.Header().Get("Content-Type") != ProblemMediaType || response.Header().Get(RequestIDHeader) != "request-123" {
		t.Fatalf("headers = %v, want problem media type and request ID", response.Header())
	}
	body := response.Body.String()
	for _, expected := range []string{
		`"type":"urn:heyblog:problem:validation_failed"`,
		`"status":422`,
		`"code":"validation_failed"`,
		`"request_id":"request-123"`,
		`"invalid_params":[{"name":"title","reason":"is required"}]`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("body = %q, want %q", body, expected)
		}
	}
	if !strings.Contains(logs.String(), "validation_failed") {
		t.Fatalf("logs = %q, want stable error code", logs.String())
	}
}

func TestErrorBoundaryHidesUnknownError(t *testing.T) {
	t.Parallel()

	router, logs := testErrorRouter(t, func(*Context) (Response, error) {
		return Response{}, errors.New("postgres://user:secret@example.test/private")
	})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/test", nil))

	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "secret") {
		t.Fatalf("response = (%d, %q), want safe internal problem", response.Code, response.Body.String())
	}
	if strings.Contains(logs.String(), "secret") {
		t.Fatalf("logs leaked internal connection information: %q", logs.String())
	}
}

func TestDescribeErrorMapsEveryApplicationKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind       apperror.Kind
		wantStatus int
		wantCode   string
	}{
		{apperror.KindBadRequest, http.StatusBadRequest, apperror.CodeBadRequest},
		{apperror.KindValidation, http.StatusUnprocessableEntity, apperror.CodeValidationFailed},
		{apperror.KindUnauthorized, http.StatusUnauthorized, apperror.CodeUnauthorized},
		{apperror.KindForbidden, http.StatusForbidden, apperror.CodeForbidden},
		{apperror.KindNotFound, http.StatusNotFound, apperror.CodeNotFound},
		{apperror.KindMethodNotAllowed, http.StatusMethodNotAllowed, apperror.CodeMethodNotAllowed},
		{apperror.KindConflict, http.StatusConflict, apperror.CodeConflict},
		{apperror.KindTooLarge, http.StatusRequestEntityTooLarge, apperror.CodeRequestTooLarge},
		{apperror.KindRateLimited, http.StatusTooManyRequests, apperror.CodeRateLimited},
		{apperror.KindUnavailable, http.StatusServiceUnavailable, apperror.CodeServiceUnavailable},
		{apperror.KindInternal, http.StatusInternalServerError, apperror.CodeInternal},
	}
	for _, test := range tests {
		descriptor := describeError(apperror.New(test.kind, "", ""))
		if descriptor.status != test.wantStatus || descriptor.code != test.wantCode {
			t.Fatalf("kind %q maps to (%d, %q), want (%d, %q)", test.kind, descriptor.status, descriptor.code, test.wantStatus, test.wantCode)
		}
	}
}

func TestErrorBoundaryLogsSafeDependencyComponent(t *testing.T) {
	t.Parallel()

	router, logs := testErrorRouter(t, func(*Context) (Response, error) {
		return Response{}, apperror.Wrap(
			componentFailure{component: "redis"},
			apperror.KindUnavailable,
			apperror.CodeServiceUnavailable,
			"service is unavailable",
			"check readiness",
		)
	})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/test", nil))

	if !strings.Contains(logs.String(), `"dependency":"redis"`) {
		t.Fatalf("logs = %q, want safe dependency component", logs.String())
	}
}

func TestErrorBoundaryLogsEveryFailedDependencyComponent(t *testing.T) {
	t.Parallel()

	router, logs := testErrorRouter(t, func(*Context) (Response, error) {
		return Response{}, apperror.Wrap(
			errors.Join(componentFailure{component: "redis"}, componentFailure{component: "database"}),
			apperror.KindUnavailable,
			apperror.CodeServiceUnavailable,
			"service is unavailable",
			"check readiness",
		)
	})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/test", nil))

	if !strings.Contains(logs.String(), `"dependencies":["database","redis"]`) {
		t.Fatalf("logs = %q, want every safe dependency component", logs.String())
	}
}

func TestErrorBoundaryDoesNotOverwriteCommittedResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	router := gin.New()
	router.Use(errorBoundary(logger), requestIDMiddleware())
	router.GET("/test", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "partial")
		_ = ctx.Error(errors.New("late failure"))
	})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/test", nil))

	if response.Code != http.StatusOK || response.Body.String() != "partial" {
		t.Fatalf("response = (%d, %q), want original committed response", response.Code, response.Body.String())
	}
	if !strings.Contains(logs.String(), `"event":"request_error"`) {
		t.Fatalf("logs = %q, want late error recorded", logs.String())
	}
}

func testErrorRouter(t *testing.T, endpoint Endpoint) (*gin.Engine, *bytes.Buffer) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	router := gin.New()
	router.Use(errorBoundary(logger), requestIDMiddleware())
	router.Any("/test", Adapt(endpoint))
	return router, &logs
}

type componentFailure struct {
	component string
}

func (failure componentFailure) Error() string {
	return "private dependency detail"
}

func (failure componentFailure) Component() string {
	return failure.component
}
