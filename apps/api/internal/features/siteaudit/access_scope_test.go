package siteaudit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"

	"heyblog-api/internal/platform/httpapi"
)

func TestLegacyRequestedAccessScopePreservesCurrentScope(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"CN_ONLY", "GLOBAL_ONLY"} {
		base := Snapshot{Name: "Original", AccessScope: "ALL"}
		proposed := Snapshot{Name: "Updated", AccessScope: "CN_ONLY"}
		current := Snapshot{Name: "Original", AccessScope: scope}
		merged, conflicts := MergeRequestedSnapshot(base, proposed, current)
		if merged.AccessScope != scope || merged.Name != "Updated" || len(conflicts) != 0 {
			t.Fatalf("merged = %#v, conflicts = %#v", merged, conflicts)
		}
		diff := BuildDiffViews(base, proposed, current, nil)
		if len(diff.Conflicts) != 0 {
			t.Fatalf("display conflicts = %#v", diff.Conflicts)
		}
	}
}

func TestSubmissionAndReviewAccessScopeHTTPContracts(t *testing.T) {
	t.Parallel()
	router := gin.New()
	api := humagin.New(router, huma.DefaultConfig("test", "1.0.0"))
	registerSubmissionRoutes(api, &Service{}, nil, nil)
	called := false
	httpapi.Register(api, huma.Operation{OperationID: "draft", Method: http.MethodPut, Path: "/draft"},
		func(_ context.Context, input *bodyInput[ReviewDraftInput]) (*bodyOutput[string], error) {
			called = true
			return &bodyOutput[string]{Body: input.Body.Site.AccessScope}, nil
		})
	site := SiteInput{Name: "Example", URL: "https://example.com", Tags: []TagInput{}, Components: []ComponentInput{}, Feeds: []FeedInput{}, Resources: []ResourceInput{}, ProgramDependencies: []ComponentInput{}}
	encoded, err := json.Marshal(SubmissionInput{Site: site})
	if err != nil {
		t.Fatal(err)
	}
	ordinaryBody := strings.Replace(string(encoded), `"site":{`, `"site":{"access_scope":"CN_ONLY",`, 1)
	for _, path := range []string{"/api/web/submissions", "/api/web/submissions/Abc123456/updates"} {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(ordinaryBody))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "access_scope") {
			t.Fatalf("%s = %d %s", path, response.Code, response.Body.String())
		}
	}
	for _, scope := range []string{"CN_ONLY", "GLOBAL_ONLY", "ALL", "unsupported"} {
		draft, err := json.Marshal(ReviewDraftInput{Site: ReviewSiteInput{SiteInput: site, AccessScope: scope}})
		if err != nil {
			t.Fatal(err)
		}
		called = false
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/draft", strings.NewReader(string(draft)))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(response, request)
		if scope == "unsupported" {
			if response.Code != http.StatusUnprocessableEntity || called {
				t.Fatalf("invalid scope = %d, called=%v", response.Code, called)
			}
		} else if response.Code != http.StatusOK || !called || !strings.Contains(response.Body.String(), scope) {
			t.Fatalf("review scope %s = %d %s", scope, response.Code, response.Body.String())
		}
	}
	schemas := api.OpenAPI().Components.Schemas.Map()
	if _, exists := schemas["SiteInput"].Properties["access_scope"]; exists {
		t.Fatal("ordinary OpenAPI input exposes access scope")
	}
	if schemas["ReviewSiteInput"].Properties["access_scope"] == nil {
		t.Fatal("review OpenAPI input omits access scope")
	}
}
