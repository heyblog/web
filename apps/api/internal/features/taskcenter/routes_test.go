package taskcenter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/platform/config"
	"heyblog-api/internal/platform/httpapi"
)

type testAuthentication struct{ user auth.User }

func (a testAuthentication) Current(context.Context, *http.Request) (auth.User, error) {
	return a.user, nil
}
func TestTaskRoutesSeparateTaskPermissionSigningAndWorkerCredentials(t *testing.T) {
	for _, tt := range []struct {
		name, method, path, body, web, bearer string
		user                                  auth.User
		status                                int
	}{
		{name: "web token required", method: "GET", path: "/api/web/management/tasks/schedules", user: auth.User{Role: auth.RoleSysAdmin}, status: 401},
		{name: "task permission required", method: "GET", path: "/api/web/management/tasks/runs", web: "web-token", user: auth.User{Role: auth.RoleAdmin}, status: 403},
		{name: "task manager cannot disable signing", method: "PUT", path: "/api/web/management/request-signing/policy", body: `{"enabled":false,"carrier":"header","ttl_seconds":300,"revision":1}`, web: "web-token", user: auth.User{Role: auth.RoleAdmin, Permissions: []auth.Permission{auth.PermissionTaskManage}}, status: 403},
		{name: "task manager cannot issue keys", method: "POST", path: "/api/web/management/request-signing/keys", body: `{"platform":"CF"}`, web: "web-token", user: auth.User{Role: auth.RoleAdmin, Permissions: []auth.Permission{auth.PermissionTaskManage}}, status: 403},
		{name: "web token is not worker token", method: "POST", path: "/api/internal/v1/tasks/claim", web: "web-token", bearer: "web-token", status: 401},
	} {
		t.Run(tt.name, func(t *testing.T) {
			router, err := httpapi.NewRouter(httpapi.Options{WebToken: "web-token", HealthcheckToken: "health-token", HTTP: config.HTTPConfig{MaxBodyBytes: 1 << 20}})
			if err != nil {
				t.Fatal(err)
			}
			RegisterRoutes(router.API, Routes{Auth: testAuthentication{tt.user}, WebToken: "web-token", WorkerToken: "worker-token"})
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(httpapi.WebTokenHeader, tt.web)
			if tt.bearer != "" {
				req.Header.Set("Authorization", "Bearer "+tt.bearer)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != tt.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}
