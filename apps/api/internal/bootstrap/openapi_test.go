package bootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/jackc/pgx/v5/pgxpool"

	"heyblog-api/internal/application/publicview"
	"heyblog-api/internal/config"
	"heyblog-api/internal/httpapi"
)

func TestApplicationOpenAPIIncludesEveryTypedBusinessRoute(t *testing.T) {
	t.Parallel()

	configuration := applicationTestConfig()
	dependencies := &stubRuntimeDependencies{
		close: func() error { return nil },
		pool:  &pgxpool.Pool{},
		views: publicview.New(nil),
	}
	handler, err := newApplicationHandler(httpapi.Options{
		Mode:               config.ModeDevelopment,
		HTTP:               configuration.HTTP,
		Logger:             discardLogger(),
		Health:             httpapi.NewHealth(dependencies, time.Second),
		HealthcheckToken:   configuration.HealthcheckToken,
		WebToken:           configuration.WebToken,
		PublicViews:        dependencies.PublicViews(),
		BodyLimitOverrides: nil,
	}, dependencies, configuration)
	if err != nil {
		t.Fatalf("newApplicationHandler() error = %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("OpenAPI status = %d, want %d", response.Code, http.StatusOK)
	}

	document, err := openapi3.NewLoader().LoadFromData(response.Body.Bytes())
	if err != nil {
		t.Fatalf("load generated OpenAPI: %v", err)
	}
	if err := document.Validate(context.Background()); err != nil {
		t.Fatalf("validate generated OpenAPI: %v", err)
	}
	if document.OpenAPI != "3.1.0" {
		t.Fatalf("OpenAPI version = %q, want 3.1.0", document.OpenAPI)
	}

	var raw struct {
		Paths      map[string]map[string]json.RawMessage `json:"paths"`
		Components struct {
			SecuritySchemes map[string]json.RawMessage `json:"securitySchemes"`
		} `json:"components"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode generated OpenAPI: %v", err)
	}
	wantedPaths := []string{
		"/ping", "/health/live", "/health/ready", "/home", "/sites", "/sites/options", "/sites/random",
		"/v1/example",
		"/sites/id/{identifier}", "/sites/id/{identifier}/icon", "/sites/custom/{customId}",
		"/auth/register", "/auth/login", "/auth/me", "/auth/refresh", "/auth/logout",
		"/auth/verify-email", "/auth/verify-email/resend", "/auth/password/forgot",
		"/auth/password/reset", "/auth/password", "/auth/github/start", "/auth/github/callback",
		"/auth/github/unbind", "/management/users", "/management/users/{id}/role",
		"/management/users/{id}/permissions", "/site-submissions",
		"/site-submissions/{shortId}/updates", "/site-submissions/{shortId}/deletions",
		"/site-submissions/{shortId}/restorations", "/site-submissions/query",
		"/site-submissions/options", "/site-submissions/site-availability",
		"/site-submissions/sites", "/site-submissions/sites/{shortId}",
		"/management/site-audits", "/management/site-audits/{auditId}",
		"/management/site-audits/{auditId}/review-draft",
		"/management/site-audits/{auditId}/review", "/internal/v1/data-import",
		"/management/api-clients", "/management/api-clients/{id}",
		"/management/api-clients/{id}/rotate", "/management/api-keys/{id}/revoke",
		"/management/api-clients/{id}/keys",
	}
	if len(raw.Paths) != len(wantedPaths) {
		t.Fatalf("documented path count = %d, want %d", len(raw.Paths), len(wantedPaths))
	}
	for _, path := range wantedPaths {
		if _, exists := raw.Paths[path]; !exists {
			t.Errorf("generated OpenAPI is missing %s", path)
		}
	}
	for _, name := range []string{"webToken", "healthBearer", "apiBearer", "accessCookie", "refreshCookie"} {
		if _, exists := raw.Components.SecuritySchemes[name]; !exists {
			t.Errorf("generated OpenAPI is missing security scheme %s", name)
		}
	}
	securityCases := []struct {
		method  string
		path    string
		schemes []string
	}{
		{method: "get", path: "/management/site-audits", schemes: []string{"webToken", "accessCookie"}},
		{method: "get", path: "/management/site-audits/{auditId}", schemes: []string{"webToken", "accessCookie"}},
		{method: "put", path: "/management/site-audits/{auditId}/review-draft", schemes: []string{"webToken", "accessCookie"}},
		{method: "delete", path: "/management/site-audits/{auditId}/review-draft", schemes: []string{"webToken", "accessCookie"}},
		{method: "post", path: "/management/site-audits/{auditId}/review", schemes: []string{"webToken", "accessCookie"}},
		{method: "post", path: "/auth/refresh", schemes: []string{"webToken", "refreshCookie"}},
		{method: "post", path: "/management/api-clients/{id}/keys", schemes: []string{"webToken", "accessCookie"}},
	}
	for _, test := range securityCases {
		var operation struct {
			Security []map[string][]string `json:"security"`
		}
		if err := json.Unmarshal(raw.Paths[test.path][test.method], &operation); err != nil {
			t.Fatalf("decode %s %s security: %v", test.method, test.path, err)
		}
		if len(operation.Security) != 1 || len(operation.Security[0]) != len(test.schemes) {
			t.Errorf("%s %s security = %#v, want one requirement containing %v", test.method, test.path, operation.Security, test.schemes)
			continue
		}
		for _, scheme := range test.schemes {
			if _, exists := operation.Security[0][scheme]; !exists {
				t.Errorf("%s %s security = %#v, want scheme %s", test.method, test.path, operation.Security, scheme)
			}
		}
	}
	exampleMethods := []string{"get", "head", "post", "put", "patch", "delete", "options"}
	for _, method := range exampleMethods {
		var operation struct {
			Security []map[string][]string `json:"security"`
		}
		if err := json.Unmarshal(raw.Paths["/v1/example"][method], &operation); err != nil {
			t.Fatalf("decode %s /v1/example security: %v", method, err)
		}
		if len(operation.Security) != 1 || !slices.Equal(operation.Security[0]["apiBearer"], []string{"example.call"}) {
			t.Errorf("%s /v1/example security = %#v, want apiBearer example.call", method, operation.Security)
		}
	}
	methods := map[string]struct{}{
		"get": {}, "head": {}, "post": {}, "put": {}, "patch": {}, "delete": {}, "options": {},
	}
	operationIDs := make(map[string]string, 54)
	operationCount := 0
	for path, pathItem := range raw.Paths {
		for method, encoded := range pathItem {
			if _, isMethod := methods[method]; !isMethod {
				continue
			}
			operationCount++
			var operation struct {
				OperationID string `json:"operationId"`
			}
			if err := json.Unmarshal(encoded, &operation); err != nil {
				t.Fatalf("decode %s %s operation: %v", method, path, err)
			}
			if operation.OperationID == "" {
				t.Errorf("%s %s has no operationId", method, path)
				continue
			}
			if previous, exists := operationIDs[operation.OperationID]; exists {
				t.Errorf("operationId %q is shared by %s and %s %s", operation.OperationID, previous, method, path)
			}
			operationIDs[operation.OperationID] = method + " " + path
		}
	}
	if operationCount != 54 {
		t.Fatalf("documented operation count = %d, want 54", operationCount)
	}
	importOperation := raw.Paths["/internal/v1/data-import"]["post"]
	if !strings.Contains(string(importOperation), `"multipart/form-data"`) ||
		!strings.Contains(string(importOperation), `"blogs"`) ||
		!strings.Contains(string(importOperation), `"taxonomy"`) {
		t.Fatalf("internal import operation does not contain its typed multipart contract")
	}
}
