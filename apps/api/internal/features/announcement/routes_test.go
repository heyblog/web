package announcement

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/publicview"
	"heyblog-api/internal/platform/config"
	"heyblog-api/internal/platform/httpapi"
)

type testAuthentication struct {
	user auth.User
	err  error
}

func (authentication testAuthentication) Current(context.Context, *http.Request) (auth.User, error) {
	return authentication.user, authentication.err
}

func TestManagementRoutesEnforceRoleAndPermission(t *testing.T) {
	base := Input{Kind: "MAIN", Title: "Title", ActionType: "NONE"}
	created, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	edited, err := json.Marshal(EditInput{Input: base, RowVersion: "1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range []struct{ method, path, body string }{
		{http.MethodGet, "/management/announcements", ""},
		{http.MethodPost, "/management/announcements", string(created)},
		{http.MethodGet, "/management/announcements/invalid", ""},
		{http.MethodPut, "/management/announcements/invalid", string(edited)},
		{http.MethodDelete, "/management/announcements/invalid", `{"rowVersion":"1"}`},
		{http.MethodPost, "/management/announcements/invalid/publish", `{"rowVersion":"1","startsAt":null,"endsAt":null}`},
		{http.MethodPost, "/management/announcements/invalid/archive", `{"rowVersion":"1"}`},
		{http.MethodGet, "/management/announcements/invalid/revisions", ""},
	} {
		t.Run(route.method+route.path, func(t *testing.T) {
			// Given an administrator without announcement management permission.
			router, err := httpapi.NewRouter(httpapi.Options{WebToken: "test-web", HealthcheckToken: "health-test", PublicViews: publicview.New(nil), HTTP: config.HTTPConfig{MaxBodyBytes: 1 << 20}})
			if err != nil {
				t.Fatal(err)
			}
			service := NewService(nil, testAuthentication{user: auth.User{Role: auth.RoleAdmin}})
			if err := RegisterRoutes(router.API, service, "test-web"); err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(httpapi.WebTokenHeader, "test-web")
			response := httptest.NewRecorder()
			// When any management operation is requested.
			router.ServeHTTP(response, request)
			// Then the request is rejected before database access.
			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestManagementRoutesRequireWebToken(t *testing.T) {
	// Given a route with an authenticated system administrator.
	router, err := httpapi.NewRouter(httpapi.Options{WebToken: "test-web", HealthcheckToken: "health-test", PublicViews: publicview.New(nil), HTTP: config.HTTPConfig{MaxBodyBytes: 1 << 20}})
	if err != nil {
		t.Fatal(err)
	}
	if err := RegisterRoutes(router.API, NewService(nil, testAuthentication{user: auth.User{Role: auth.RoleSysAdmin}}), "test-web"); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	// When the Web-token guard receives no token.
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/management/announcements", nil))
	// Then the caller is rejected before user or database access.
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
}
