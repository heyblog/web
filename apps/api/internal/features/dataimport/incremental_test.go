package dataimport

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func incrementalTestBundles(t *testing.T) Bundles {
	t.Helper()
	blogs, graph := testBundleJSON()
	bundles, err := DecodeBundles(blogs, graph)
	if err != nil {
		t.Fatal(err)
	}
	bundles.Mode = ImportIncremental
	bundles.Blogs.Inputs[0].Kind = "nodes"
	bundles.Blogs.Inputs[1].Kind = "edges"
	bundles.Graph.Inputs = bundles.Blogs.Inputs
	for index := range bundles.Blogs.Blogs {
		blog := &bundles.Blogs.Blogs[index]
		blog.Visibility = "VISIBLE"
		blog.VisibilityReason = nil
		blog.Sitemap, blog.LinkPage, blog.MainTag, blog.Architecture = nil, nil, nil, nil
		blog.SubTags = []LegacyTag{}
		blog.Origins = []LegacyOrigin{{SourceKey: friendGraphSourceKey, ExternalReference: blog.ID, FirstDiscoveredAt: blog.JoinedAt, Metadata: OriginMetadata{InputKinds: []string{"nodes"}, ExternalReferences: []string{blog.ID}}}}
	}
	return bundles
}

func TestIncrementalContractAcceptsActualSourcesAndRejectsUnsupportedProfiles(t *testing.T) {
	t.Parallel()
	// Given
	bundles := incrementalTestBundles(t)
	blogs, err := json.Marshal(bundles.Blogs)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := json.Marshal(bundles.Graph)
	if err != nil {
		t.Fatal(err)
	}
	// When
	decoded, err := DecodeBundles(blogs, graph)
	// Then
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(decoded, sequenceGenerator("AAAAAAAAA", "BBBBBBBBB"))
	if err != nil || len(plan.Sources) != 1 || plan.Sources[0].Key != friendGraphSourceKey {
		t.Fatalf("plan=%#v err=%v", plan.Sources, err)
	}
	bundles.Blogs.Blogs[0].Sitemap = new("/sitemap.xml")
	if err := validateImportMode(bundles); err == nil {
		t.Fatal("resource silently accepted in incremental mode")
	}
}

func TestIncrementalResolutionPreservesStoredIdentityAndCanonicalTarget(t *testing.T) {
	t.Parallel()
	// Given
	bundles := incrementalTestBundles(t)
	plan, err := BuildPlan(bundles, sequenceGenerator("AAAAAAAAA", "BBBBBBBBB"))
	if err != nil {
		t.Fatal(err)
	}
	stored := []dbgen.ListIncrementalSitesRow{{ID: mustUUID(testSiteIDTwo), ShortID: "AAAAAAAAA", NormalizedHost: "other.example", Scheme: "https", BasePath: "/changed"}}
	// When
	resolved, counts, err := resolveIncremental(plan, stored, nil, sequenceGenerator("CCCCCCCCC"))
	// Then
	if err != nil {
		t.Fatal(err)
	}
	if counts.Sites != 1 || counts.MatchedSites != 1 || len(resolved.Sites) != 1 || resolved.Sites[0].ShortID != "CCCCCCCCC" {
		t.Fatalf("sites=%#v counts=%#v", resolved.Sites, counts)
	}
	if len(resolved.FriendLinks) != 1 || resolved.FriendLinks[0].TargetURL != "https://other.example/changed" {
		t.Fatalf("links=%#v", resolved.FriendLinks)
	}
}

func TestIncrementalResolutionSkipsAllExistingPairs(t *testing.T) {
	t.Parallel()
	// Given
	bundles := incrementalTestBundles(t)
	plan, err := BuildPlan(bundles, sequenceGenerator("AAAAAAAAA", "BBBBBBBBB"))
	if err != nil {
		t.Fatal(err)
	}
	stored := []dbgen.ListIncrementalSitesRow{
		{ID: mustUUID(testSiteIDOne), ShortID: "AAAAAAAAA", NormalizedHost: "example.com", Scheme: "https", BasePath: "/blog"},
		{ID: mustUUID(testSiteIDTwo), ShortID: "BBBBBBBBB", NormalizedHost: "other.example", Scheme: "http", BasePath: "/"},
	}
	pairs := []dbgen.ListIncrementalFriendPairsRow{{SourceSiteID: mustUUID(testSiteIDOne), TargetHost: "other.example"}}
	// When
	resolved, counts, err := resolveIncremental(plan, stored, pairs, sequenceGenerator("CCCCCCCCC"))
	// Then
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved.Sites) != 0 || len(resolved.Feeds) != 0 || len(resolved.FriendLinks) != 0 || counts.MatchedSites != 2 || counts.ExistingFriendLinks != 1 {
		t.Fatalf("resolved=%#v counts=%#v", resolved, counts)
	}
}

func TestRoutePassesExplicitIncrementalMode(t *testing.T) {
	t.Parallel()
	// Given
	bundles := incrementalTestBundles(t)
	blogs, err := json.Marshal(bundles.Blogs)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := json.Marshal(bundles.Graph)
	if err != nil {
		t.Fatal(err)
	}
	request := multipartModeRequest(t, "incremental", blogs, graph)
	operation := &recordingOperation{}
	recorder := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	// When
	newImportTestRouter(t, operation).ServeHTTP(recorder, request)
	// Then
	if recorder.Code != http.StatusOK || operation.bundles.Mode != ImportIncremental {
		t.Fatalf("status=%d body=%s mode=%s", recorder.Code, recorder.Body.String(), operation.bundles.Mode)
	}
}

type incrementalRecordingStore struct{ plan Plan }

func (store *incrementalRecordingStore) Import(context.Context, Plan) (Counts, error) {
	return Counts{}, ErrDirectoryNotEmpty
}
func (store *incrementalRecordingStore) ImportIncremental(_ context.Context, plan Plan, _ func() (string, error)) (Counts, error) {
	store.plan = plan
	return Counts{MatchedSites: len(plan.Sites)}, nil
}

func TestServiceDispatchesIncrementalMode(t *testing.T) {
	t.Parallel()
	// Given
	store := &incrementalRecordingStore{}
	service := NewService(store, sequenceGenerator("AAAAAAAAA", "BBBBBBBBB"))
	// When
	counts, err := service.Import(context.Background(), incrementalTestBundles(t))
	// Then
	if err != nil || counts.MatchedSites != 2 || len(store.plan.Sources) != 1 {
		t.Fatalf("counts=%#v err=%v", counts, err)
	}
}

func multipartModeRequest(t *testing.T, mode string, blogs, graph []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("mode", mode); err != nil {
		t.Fatal(err)
	}
	for _, file := range []struct {
		name     string
		contents []byte
	}{{"blogs", blogs}, {"graph", graph}} {
		part, err := writer.CreateFormFile(file.name, file.name+".json")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(file.contents); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, Path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+testImportToken)
	return request
}
