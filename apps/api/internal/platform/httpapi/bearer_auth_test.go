package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBearerAuthorizationRejectsBeforeCallingEndpoint(t *testing.T) {
	t.Parallel()

	endpointCalled := false
	handler := Adapt(Chain(func(*Context) (Response, error) {
		endpointCalled = true
		return NoContent(http.StatusNoContent), nil
	}, BearerAuthorization("expected-token", "one-time-import")))
	router := gin.New()
	router.Use(errorBoundary(slog.New(slog.NewTextHandler(io.Discard, nil))))
	router.POST("/internal", handler)

	request := httptest.NewRequest(http.MethodPost, "/internal", nil)
	request.Header.Set("Authorization", "Bearer wrong-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if got := response.Header().Get("WWW-Authenticate"); got != `Bearer realm="one-time-import"` {
		t.Fatalf("WWW-Authenticate = %q, want one-time import challenge", got)
	}
	if endpointCalled {
		t.Fatal("endpoint was called after authentication failed")
	}
}

func TestBearerAuthorizationAcceptsCaseInsensitiveScheme(t *testing.T) {
	t.Parallel()

	handler := Adapt(Chain(func(*Context) (Response, error) {
		return NoContent(http.StatusNoContent), nil
	}, BearerAuthorization("expected-token", "one-time-import")))
	router := gin.New()
	router.POST("/internal", handler)
	request := httptest.NewRequest(http.MethodPost, "/internal", nil)
	request.Header.Set("Authorization", "bearer expected-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}
