package httpapi

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

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
