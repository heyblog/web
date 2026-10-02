package httpapi

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type deadlineResponseRecorder struct {
	*httptest.ResponseRecorder
	readDeadline  time.Time
	writeDeadline time.Time
}

func (recorder *deadlineResponseRecorder) SetReadDeadline(deadline time.Time) error {
	recorder.readDeadline = deadline
	return nil
}

func (recorder *deadlineResponseRecorder) SetWriteDeadline(deadline time.Time) error {
	recorder.writeDeadline = deadline
	return nil
}

func TestChainStopsBeforeNextEndpointWhenMiddlewareFails(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("denied")
	endpointCalled := false
	endpoint := func(*Context) (Response, error) {
		endpointCalled = true
		return NoContent(http.StatusNoContent), nil
	}
	failingMiddleware := func(next Endpoint) Endpoint {
		return func(*Context) (Response, error) {
			return Response{}, wantErr
		}
	}

	_, err := Chain(endpoint, failingMiddleware)(nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Chain() error = %v, want %v", err, wantErr)
	}
	if endpointCalled {
		t.Fatal("endpoint was called after middleware returned an error")
	}
}

func TestAdaptRejectsInvalidSuccessResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(errorBoundary(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))), requestIDMiddleware())
	router.GET("/result", Adapt(func(*Context) (Response, error) {
		return Response{}, nil
	}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/result", nil))

	if response.Code != http.StatusInternalServerError || response.Header().Get("Content-Type") != ProblemMediaType {
		t.Fatalf("response = (%d, %q), want invalid result mapped to internal problem", response.Code, response.Header().Get("Content-Type"))
	}
}

func TestAdaptWritesPreparedJSONResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/result", Adapt(func(*Context) (Response, error) {
		return JSON(http.StatusCreated, map[string]string{"status": "created"})
	}))

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/result", nil))

	if response.Code != http.StatusCreated || response.Body.String() != `{"status":"created"}` {
		t.Fatalf("response = (%d, %q), want prepared JSON", response.Code, response.Body.String())
	}
}

func TestContextSetsConnectionDeadlinesThroughResponseController(t *testing.T) {
	t.Parallel()

	deadline := time.Now().Add(10 * time.Minute).Round(time.Millisecond)
	router := gin.New()
	router.POST("/deadline", Adapt(func(ctx *Context) (Response, error) {
		if err := ctx.SetReadDeadline(deadline); err != nil {
			return Response{}, err
		}
		if err := ctx.SetWriteDeadline(deadline); err != nil {
			return Response{}, err
		}
		return NoContent(http.StatusNoContent), nil
	}))
	recorder := &deadlineResponseRecorder{ResponseRecorder: httptest.NewRecorder()}
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/deadline", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if !recorder.readDeadline.Equal(deadline) || !recorder.writeDeadline.Equal(deadline) {
		t.Fatalf("deadlines = (%s, %s), want %s", recorder.readDeadline, recorder.writeDeadline, deadline)
	}
}
