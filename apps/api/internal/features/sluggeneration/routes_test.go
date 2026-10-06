package sluggeneration

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/publicview"
	"heyblog-api/internal/platform/config"
	"heyblog-api/internal/platform/httpapi"
)

func testRouter(t *testing.T, service *Service) *httpapi.Router {
	t.Helper()
	router, err := httpapi.NewRouter(httpapi.Options{WebToken: "test-web", HealthcheckToken: "test-health", PublicViews: publicview.New(nil, nil), HTTP: config.HTTPConfig{MaxBodyBytes: 1 << 20}})
	if err != nil {
		t.Fatal(err)
	}
	if err := RegisterRoutes(router.API, service, "test-web"); err != nil {
		t.Fatal(err)
	}
	return router
}

func TestRoutesEnforceBothWebAndUserAuthorizationBeforeBody(t *testing.T) {
	routes := []struct{ method, path, body string }{
		{"POST", "/management/taxonomy/slug-generation", `{"name":"Go"}`},
		{"GET", "/management/system-settings", ""},
		{"PUT", "/management/system-settings", `{"model_id":"deepseek/deepseek-flash","expected_revision":"0"}`},
		{"GET", "/management/system-settings/models", ""},
		{"POST", "/management/taxonomy/slug-jobs", `{"selection":{"kind":"invalid"}}`},
		{"GET", "/management/taxonomy/slug-jobs", ""},
		{"GET", "/management/taxonomy/slug-jobs/019ded7f-4b91-702b-9f01-acb5ea1c99d8", ""},
		{"POST", "/management/taxonomy/slug-jobs/019ded7f-4b91-702b-9f01-acb5ea1c99d8/control", `{"action":"resume"}`},
		{"PATCH", "/management/taxonomy/slug-jobs/019ded7f-4b91-702b-9f01-acb5ea1c99d8/items", `{"items":[],"expected_revision":"1"}`},
		{"POST", "/management/taxonomy/slug-jobs/019ded7f-4b91-702b-9f01-acb5ea1c99d8/apply", `{"tag_ids":[],"expected_revision":"1"}`},
	}
	for _, route := range routes {
		for _, test := range []struct {
			name, token string
			user        auth.User
			authErr     error
			status      int
		}{
			{name: "no-web-token", user: auth.User{Role: auth.RoleSysAdmin}, status: 401},
			{name: "no-session", token: "test-web", authErr: &auth.AuthError{StatusCode: 401, Code: "unauthorized", Message: "authentication required"}, status: 401},
			{name: "admin-no-permission", token: "test-web", user: auth.User{Role: auth.RoleAdmin}, status: 403},
			{name: "user-with-permission", token: "test-web", user: auth.User{Role: auth.RoleUser, Permissions: []auth.Permission{auth.PermissionTaxonomyManage}}, status: 403},
		} {
			t.Run(route.method+route.path+test.name, func(t *testing.T) {
				service, _, provider, guard := testService()
				service.auth = testAuthentication{user: test.user, err: test.authErr}
				router := testRouter(t, service)
				request := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
				request.Header.Set(httpapi.WebTokenHeader, test.token)
				request.Header.Set("Content-Type", "application/json")
				result := httptest.NewRecorder()
				router.ServeHTTP(result, request)
				if result.Code != test.status || provider.calls != 0 || provider.modelCalls != 0 || guard.allowed != 0 {
					t.Fatalf("status=%d body=%s", result.Code, result.Body.String())
				}
			})
		}
	}
}

func TestTaxonomyManagerMayGenerateButCannotReadSystemSettings(t *testing.T) {
	service, _, _, _ := testService()
	service.auth = testAuthentication{user: auth.User{ID: "user", Role: auth.RoleAdmin, Permissions: []auth.Permission{auth.PermissionTaxonomyManage}}}
	router := testRouter(t, service)
	for _, test := range []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/management/taxonomy/slug-generation", `{"name":"Go Language"}`, 200},
		{"GET", "/management/system-settings", "", 403},
		{"GET", "/management/system-settings/models", "", 403},
	} {
		request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
		request.Header.Set(httpapi.WebTokenHeader, "test-web")
		request.Header.Set("Content-Type", "application/json")
		result := httptest.NewRecorder()
		router.ServeHTTP(result, request)
		if result.Code != test.status {
			t.Fatalf("%s status=%d body=%s", test.path, result.Code, result.Body.String())
		}
	}
}

func TestGenerationRejectsProxyFieldsAndOversizedBodies(t *testing.T) {
	for _, body := range []string{`{"name":"Go","model":"attacker-model"}`, `{"name":"Go","messages":[]}`, `{"name":"Go","base_url":"https://attacker.test"}`, `{"name":"` + strings.Repeat("a", 9000) + `"}`} {
		service, _, provider, guard := testService()
		router := testRouter(t, service)
		request := httptest.NewRequest("POST", "/management/taxonomy/slug-generation", strings.NewReader(body))
		request.Header.Set(httpapi.WebTokenHeader, "test-web")
		request.Header.Set("Content-Type", "application/json")
		result := httptest.NewRecorder()
		router.ServeHTTP(result, request)
		if result.Code != 413 && result.Code != 422 && result.Code != 400 {
			t.Fatalf("unsafe request accepted status=%d body=%s", result.Code, result.Body.String())
		}
		if provider.calls != 0 || guard.charged != 0 {
			t.Fatal("invalid request reached provider")
		}
	}
}

func TestGenerationRateLimitHasRetryHeader(t *testing.T) {
	service, _, provider, guard := testService()
	guard.err = rateFailure("slug_rate_limited", 42)
	router := testRouter(t, service)
	request := httptest.NewRequest("POST", "/management/taxonomy/slug-generation", strings.NewReader(`{"name":"Go"}`))
	request.Header.Set(httpapi.WebTokenHeader, "test-web")
	request.Header.Set("Content-Type", "application/json")
	result := httptest.NewRecorder()
	router.ServeHTTP(result, request)
	if result.Code != http.StatusTooManyRequests || result.Header().Get("Retry-After") != "42" || provider.calls != 0 {
		t.Fatalf("rate result=%d headers=%v body=%s", result.Code, result.Header(), result.Body.String())
	}
}
