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

	"heyblog-api/internal/features/publicview"
	"heyblog-api/internal/platform/config"
	"heyblog-api/internal/platform/httpapi"
)

func TestApplicationOpenAPIIncludesEveryTypedBusinessRoute(t *testing.T) {
	t.Parallel()

	configuration := applicationTestConfig()
	dependencies := &stubRuntimeDependencies{
		close: func() error { return nil },
		pool:  &pgxpool.Pool{},
		views: publicview.New(nil, nil),
	}
	handler, err := newApplicationHandler(httpapi.Options{
		Mode:               config.ModeDevelopment,
		HTTP:               configuration.HTTP,
		Logger:             discardLogger(),
		Health:             httpapi.NewHealth(dependencies, time.Second),
		HealthcheckToken:   configuration.HealthcheckToken,
		WebToken:           configuration.WebToken,
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
		"/api/web/account/profile/confirm", "/api/web/account/profile", "/api/web/account/username",
		"/api/web/management/users/{id}", "/api/web/management/users/{id}/profile", "/api/web/management/users/{id}/status",
		"/api/web/management/users/{id}/sessions/revoke", "/api/web/management/users/{id}/password-reset", "/api/web/management/users/{id}/verification",
		"/api/web/management/database-backup/restore-jobs/current", "/api/web/management/database-backup/restore-jobs/{id}", "/api/web/management/database-backup/restore-jobs/{id}/cancel",
		"/api/web/management/database-backup/export", "/api/web/management/database-backup/inspect", "/api/web/management/database-backup/restore",
		"/api/web/management/database-backup/uploads", "/api/web/management/database-backup/uploads/current", "/api/web/management/database-backup/uploads/{id}",
		"/api/web/management/database-backup/uploads/{id}/parts/{number}", "/api/web/management/database-backup/uploads/{id}/complete",
		"/api/web/public/sitemap",
		"/api/web/public/sites/id/{shortId}/outbound", "/api/web/public/site-metrics/impressions",
		"/api/web/public/announcements", "/api/web/public/announcements/banner", "/api/web/public/announcements/{id}",
		"/api/web/management/announcements", "/api/web/management/announcements/{id}",
		"/api/web/management/announcements/{id}/publish", "/api/web/management/announcements/{id}/archive", "/api/web/management/announcements/{id}/revisions",
		"/api/web/public/sites/graph", "/api/web/public/sites/id/{identifier}/graph",
		"/api/web/public/ping", "/health/live", "/health/ready", "/api/web/public/home", "/api/web/public/sites", "/api/web/public/sites/options", "/api/web/public/sites/random",
		"/api/external/v1/example",
		"/api/web/public/sites/id/{identifier}", "/api/web/public/sites/id/{identifier}/icon", "/api/web/public/sites/id/{identifier}/metadata", "/api/web/public/sites/custom/{customId}",
		"/api/web/auth/register", "/api/web/auth/login", "/api/web/auth/me", "/api/web/auth/refresh", "/api/web/auth/logout",
		"/api/web/auth/verify-email", "/api/web/auth/verify-email/resend", "/api/web/auth/password/forgot",
		"/api/web/auth/password/reset", "/api/web/auth/password", "/api/web/auth/github/start", "/api/web/auth/github/callback",
		"/api/web/auth/blogsclub/start", "/api/web/auth/blogsclub/callback", "/api/web/auth/blogsclub/unbind",
		"/api/web/auth/github/unbind", "/api/web/management/users", "/api/web/management/users/{id}/role",
		"/api/web/management/users/{id}/permissions", "/api/web/submissions",
		"/api/web/submissions/{shortId}/updates", "/api/web/submissions/{shortId}/deletions",
		"/api/web/submissions/{shortId}/restorations", "/api/web/submissions/query",
		"/api/web/submissions/options", "/api/web/submissions/site-availability",
		"/api/web/submissions/sites", "/api/web/submissions/sites/{shortId}",
		"/api/web/management/site-audits", "/api/web/management/site-audits/{auditId}",
		"/api/web/management/site-audits/{auditId}/review-draft",
		"/api/web/management/site-audits/{auditId}/review", "/api/internal/v1/data-import",
		"/api/web/management/api-clients", "/api/web/management/api-clients/{id}",
		"/api/web/management/api-clients/{id}/rotate", "/api/web/management/api-keys/{id}/revoke",
		"/api/web/management/api-clients/{id}/keys",
		"/api/web/account/sites", "/api/web/account/sites/{shortId}",
		"/api/web/account/site-submissions", "/api/web/account/site-submissions/{auditId}",
		"/api/web/account/sites/{shortId}/updates", "/api/web/account/sites/{shortId}/friend-link-submissions",
		"/api/web/account/sites/{shortId}/friend-links", "/api/web/account/sites/{shortId}/friend-links/{targetShortId}",
		"/api/web/account/sites/{shortId}/friend-links/by-host/{targetHost}",
		"/api/web/account/sites/{shortId}/friend-link-requests/{requestId}",
		"/api/web/account/sites/{shortId}/claim-context", "/api/web/account/site-claims/{claimId}/regenerate",
		"/api/web/account/site-claims", "/api/web/account/site-claims/{claimId}", "/api/web/account/site-claims/{claimId}/check",
		"/api/web/account/site-claim-attachments/settings", "/api/web/account/site-claim-attachments/{id}/download", "/api/web/management/site-claim-attachments/{id}/download",
		"/api/web/account/site-claim-attachments", "/api/web/account/site-claim-attachments/{id}", "/api/web/account/site-claim-attachments/{id}/complete",
		"/api/web/management/site-claims", "/api/web/management/site-claims/{claimId}",
		"/api/web/management/site-claims/{claimId}/review", "/api/web/management/site-ownership/{shortId}",
		"/api/web/management/taxonomy/tags", "/api/web/management/taxonomy/tags/{id}",
		"/api/web/management/taxonomy/cascades",
		"/api/web/management/taxonomy/changes/preview", "/api/web/management/taxonomy/changes/apply",
	}
	if len(raw.Paths) != len(wantedPaths) {
		t.Fatalf("documented path count = %d, want %d", len(raw.Paths), len(wantedPaths))
	}
	for _, path := range wantedPaths {
		if _, exists := raw.Paths[path]; !exists {
			t.Errorf("generated OpenAPI is missing %s", path)
		}
	}
	for _, oldPath := range []string{"/ping", "/home", "/sites", "/auth/me", "/site-submissions/options", "/account/sites", "/management/users", "/v1/example", "/internal/v1/data-import"} {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			legacy := httptest.NewRecorder()
			handler.ServeHTTP(legacy, httptest.NewRequest(method, oldPath, nil))
			if legacy.Code != http.StatusNotFound {
				t.Errorf("legacy %s %s status = %d, want 404", method, oldPath, legacy.Code)
			}
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
		{method: "get", path: "/api/web/public/sitemap", schemes: []string{"webToken"}},
		{method: "get", path: "/api/web/management/announcements", schemes: []string{"webToken", "accessCookie"}},
		{method: "post", path: "/api/web/management/announcements/{id}/publish", schemes: []string{"webToken", "accessCookie"}},
		{method: "get", path: "/api/web/management/site-audits", schemes: []string{"webToken", "accessCookie"}},
		{method: "get", path: "/api/web/management/site-audits/{auditId}", schemes: []string{"webToken", "accessCookie"}},
		{method: "put", path: "/api/web/management/site-audits/{auditId}/review-draft", schemes: []string{"webToken", "accessCookie"}},
		{method: "delete", path: "/api/web/management/site-audits/{auditId}/review-draft", schemes: []string{"webToken", "accessCookie"}},
		{method: "post", path: "/api/web/management/site-audits/{auditId}/review", schemes: []string{"webToken", "accessCookie"}},
		{method: "post", path: "/api/web/management/taxonomy/changes/apply", schemes: []string{"webToken", "accessCookie"}},
		{method: "post", path: "/api/web/auth/refresh", schemes: []string{"webToken", "refreshCookie"}},
		{method: "post", path: "/api/web/management/api-clients/{id}/keys", schemes: []string{"webToken", "accessCookie"}},
		{method: "get", path: "/api/web/account/sites", schemes: []string{"webToken", "accessCookie"}},
		{method: "post", path: "/api/web/account/sites/{shortId}/updates", schemes: []string{"webToken", "accessCookie"}},
		{method: "delete", path: "/api/web/account/sites/{shortId}/friend-links/by-host/{targetHost}", schemes: []string{"webToken", "accessCookie"}},
		{method: "post", path: "/api/web/account/site-claims/{claimId}/check", schemes: []string{"webToken", "accessCookie"}},
		{method: "post", path: "/api/web/management/site-claims/{claimId}/review", schemes: []string{"webToken", "accessCookie"}},
		{method: "delete", path: "/api/web/management/site-ownership/{shortId}", schemes: []string{"webToken", "accessCookie"}},
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
		if err := json.Unmarshal(raw.Paths["/api/external/v1/example"][method], &operation); err != nil {
			t.Fatalf("decode %s /v1/example security: %v", method, err)
		}
		if len(operation.Security) != 1 || !slices.Equal(operation.Security[0]["apiBearer"], []string{"example.call"}) {
			t.Errorf("%s /v1/example security = %#v, want apiBearer example.call", method, operation.Security)
		}
	}
	methods := map[string]struct{}{
		"get": {}, "head": {}, "post": {}, "put": {}, "patch": {}, "delete": {}, "options": {},
	}
	operationIDs := make(map[string]string, 114)
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
	if operationCount != 134 {
		t.Fatalf("documented operation count = %d, want 134", operationCount)
	}
	importOperation := raw.Paths["/api/internal/v1/data-import"]["post"]
	if !strings.Contains(string(importOperation), `"multipart/form-data"`) ||
		!strings.Contains(string(importOperation), `"blogs"`) ||
		!strings.Contains(string(importOperation), `"taxonomy"`) {
		t.Fatalf("internal import operation does not contain its typed multipart contract")
	}
}
