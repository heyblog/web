package taxonomy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/publicview"
	"heyblog-api/internal/platform/config"
	"heyblog-api/internal/platform/httpapi"
)

type testActor struct{ user auth.User }

func (a testActor) Current(context.Context, *http.Request) (auth.User, error) { return a.user, nil }

func TestManagementRoutesEnforceBothGuards(t *testing.T) {
	for _, route := range []struct{ method, path, body string }{
		{"GET", "/tags", ""},
		{"POST", "/cascades", `{"scope":"SITE","primary_id":"id","secondary_id":"id","taxonomy_key":"test/self","expected_revision":"version"}`},
		{"POST", "/tags", `{"name":"New","slug":"new","description":"","expected_revision":"version"}`},
		{"PUT", "/tags/id", `{"name":"New","slug":"new","description":"","is_enabled":true,"expected_revision":"version"}`},
		{"DELETE", "/tags/id", `{"expected_revision":"version"}`},
		{"POST", "/changes/preview", `{"kind":"merge","source_id":"id","expected_revision":"version"}`},
		{"POST", "/changes/apply", `{"kind":"merge","source_id":"id","expected_revision":"version"}`},
	} {
		t.Run(route.method+route.path, func(t *testing.T) {
			router, err := httpapi.NewRouter(httpapi.Options{WebToken: "web-test", HealthcheckToken: "health-test", PublicViews: publicview.New(nil), HTTP: config.HTTPConfig{MaxBodyBytes: 1 << 20}})
			if err != nil {
				t.Fatal(err)
			}
			if err = RegisterRoutes(router.API, NewService(nil, testActor{user: auth.User{Role: auth.RoleAdmin}}), "web-test"); err != nil {
				t.Fatal(err)
			}
			for _, token := range []string{"", "web-test"} {
				req := httptest.NewRequest(route.method, "/management/taxonomy"+route.path, strings.NewReader(route.body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set(httpapi.WebTokenHeader, token)
				response := httptest.NewRecorder()
				router.ServeHTTP(response, req)
				want := http.StatusUnauthorized
				if token != "" {
					want = http.StatusForbidden
				}
				if response.Code != want {
					t.Fatalf("status=%d want=%d body=%s", response.Code, want, response.Body.String())
				}
			}
		})
	}
}
func TestTaxonomyPermissionCannotElevateOrdinaryRole(t *testing.T) {
	for _, user := range []auth.User{{Role: auth.RoleSysAdmin}, {Role: auth.RoleAdmin, Permissions: []auth.Permission{auth.PermissionTaxonomyManage}}, {Role: auth.RoleUser, Permissions: []auth.Permission{auth.PermissionTaxonomyManage}}} {
		s := NewService(nil, testActor{user: user})
		err := s.authorize(t.Context(), httptest.NewRequest("GET", "/", nil))
		want := user.Role != auth.RoleUser
		if (err == nil) != want {
			t.Fatalf("role=%s error=%v", user.Role, err)
		}
	}
}
