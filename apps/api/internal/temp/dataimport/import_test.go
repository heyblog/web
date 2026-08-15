package dataimport

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testSiteIDOne = "01900000-0000-7000-8000-000000000001"
	testSiteIDTwo = "01900000-0000-7000-8000-000000000002"
	testTagIDMain = "01900000-0000-7000-8000-000000000010"
	testTagIDSub  = "01900000-0000-7000-8000-000000000011"
	testProgramID = "01900000-0000-7000-8000-000000000020"
	testStackID   = "01900000-0000-7000-8000-000000000021"
)

func testBundleJSON() ([]byte, []byte) {
	inputs := fmt.Sprintf(`[
    {"kind":"zhblogs","file":"zhblogs.json","sha256":"%s","count":2},
    {"kind":"nodes","file":"nodes.jsonl","sha256":"%s","count":2},
    {"kind":"edges","file":"edges.jsonl","sha256":"%s","count":1},
    {"kind":"outbound","file":"outbound.json","sha256":"%s","count":1,"edge_count":1}
  ]`, strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("d", 64))
	blogs := fmt.Sprintf(`{
  "format": "heyblog.data-import.blogs",
  "version": 2,
  "generated_at": "2026-08-15T12:00:00Z",
  "inputs": %s,
  "count": 2,
  "blogs": [
    {
      "id": "%s",
      "name": "Example",
      "url": "https://example.com/blog",
      "summary": "Example summary",
      "feeds": [{"url":"/blog/feed.xml","name":"Default","is_default":true,"format":"ATOM"}],
      "sitemap": "/sitemap.xml",
      "link_page": "https://links.example/page",
      "joined_at": "2025-01-01T00:00:00Z",
      "created_at": "2025-01-01T00:00:00Z",
      "updated_at": "2026-01-01T00:00:00Z",
      "access_scope": "GLOBAL_ONLY",
      "visibility": "VISIBLE",
      "visibility_reason": null,
      "origins": [
        {"source_key":"ZHBLOGS_OLD","external_reference":"%s","first_discovered_at":"2025-01-01T00:00:00Z","metadata":{"input_kinds":["zhblogs"],"external_references":["%s"]}},
        {"source_key":"HEYBLOG_OLD","external_reference":"10","first_discovered_at":"2025-01-02T00:00:00Z","metadata":{"input_kinds":["nodes"],"external_references":["10"]}}
      ],
      "main_tag": {"id":"%s","name":"技术","machine_key":"technology","description":"Tech","is_enabled":true},
      "sub_tags": [{"id":"%s","name":"开源","machine_key":null,"description":null,"is_enabled":true}],
      "architecture": {
        "program": {"id":"%s","name":"Astro","name_normalized":"astro","is_open_source":true,"website_url":"https://astro.build","repo_url":"https://github.com/withastro/astro","is_enabled":true},
        "technology_stacks": [{"id":"%s","category":"LANGUAGE","name":"Go","name_normalized":"go","catalog":null}]
      }
    },
    {
      "id": "%s",
      "name": "Other",
      "url": "http://other.example/",
      "summary": "该博客暂无描述",
      "feeds": [],
      "sitemap": null,
      "link_page": null,
      "joined_at": "2025-02-01T00:00:00Z",
      "created_at": "2025-02-01T00:00:00Z",
      "updated_at": "2026-02-01T00:00:00Z",
      "access_scope": "ALL",
      "visibility": "HIDDEN",
      "visibility_reason": "FRIEND_LINK_DISCOVERY_PENDING_REVIEW",
      "origins": [
        {"source_key":"WEB_SUBMIT","external_reference":"submission-2","first_discovered_at":"2025-02-01T00:00:00Z","metadata":{"input_kinds":["zhblogs"],"external_references":["submission-2"]}},
        {"source_key":"FRIEND_LINK_DISCOVERY","external_reference":"other.example","first_discovered_at":"2026-08-15T12:00:00Z","metadata":{"input_kinds":["graph"],"external_references":["other.example"]}}
      ],
      "main_tag": null,
      "sub_tags": [],
      "architecture": null
    }
  ]
}`, inputs, testSiteIDOne, testSiteIDOne, testSiteIDOne, testTagIDMain, testTagIDSub, testProgramID, testStackID, testSiteIDTwo)
	graph := fmt.Sprintf(`{
  "format": "heyblog.data-import.graph",
  "version": 2,
  "generated_at": "2026-08-15T12:00:00Z",
  "inputs": %s,
  "node_count": 2,
  "count": 1,
  "edge_count": 1,
  "links": [{"source":"example.com","destinations":["http://other.example/"]}]
}`, inputs)
	return []byte(blogs), []byte(graph)
}

func TestDecodeBundlesRejectsUnknownFieldsAndCountMismatch(t *testing.T) {
	t.Parallel()

	blogs, friends := testBundleJSON()
	tests := map[string][]byte{
		"unknown field":      []byte(strings.Replace(string(blogs), `"count": 2,`, `"count": 2, "unexpected": true,`, 1)),
		"count mismatch":     []byte(strings.Replace(string(blogs), "\"count\": 2,\n  \"blogs\"", "\"count\": 1,\n  \"blogs\"", 1)),
		"legacy format":      []byte(strings.Replace(string(blogs), `heyblog.data-import.blogs`, `zhblogs.blogs`, 1)),
		"missing visibility": []byte(strings.Replace(string(blogs), `      "visibility": "VISIBLE",`, "", 1)),
		"null feeds":         []byte(strings.Replace(string(blogs), `"feeds": []`, `"feeds": null`, 1)),
		"invalid visibility": []byte(strings.Replace(string(blogs), `"visibility": "VISIBLE"`, `"visibility": "REMOVED"`, 1)),
		"invalid access":     []byte(strings.Replace(string(blogs), `"access_scope": "ALL"`, `"access_scope": "NON_CN_ONLY"`, 1)),
	}
	for name, malformedBlogs := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeBundles(malformedBlogs, friends); err == nil {
				t.Fatal("DecodeBundles() error = nil, want cleaned-contract error")
			}
		})
	}
}

func TestBuildPlanRejectsRelationalConstraintViolationsBeforeStore(t *testing.T) {
	t.Parallel()

	blogs, friends := testBundleJSON()
	bundles, err := DecodeBundles(blogs, friends)
	if err != nil {
		t.Fatalf("DecodeBundles() error = %v", err)
	}
	tests := map[string]func(Bundles) Bundles{
		"duplicate feed location": func(candidate Bundles) Bundles {
			candidate.Blogs.Blogs[0].Feeds = append(candidate.Blogs.Blogs[0].Feeds, candidate.Blogs.Blogs[0].Feeds[0])
			return candidate
		},
		"multiple default feeds": func(candidate Bundles) Bundles {
			candidate.Blogs.Blogs[0].Feeds = append(candidate.Blogs.Blogs[0].Feeds, LegacyFeed{
				URL: "https://example.com/second.xml", Name: "Second", IsDefault: true,
			})
			return candidate
		},
		"duplicate friend target": func(candidate Bundles) Bundles {
			candidate.Graph.Links[0].Destinations = append(
				candidate.Graph.Links[0].Destinations,
				candidate.Graph.Links[0].Destinations[0],
			)
			return candidate
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := BuildPlan(mutate(bundles), sequenceGenerator("AAAAAAAAA", "BBBBBBBBB")); err == nil {
				t.Fatal("BuildPlan() error = nil, want plan validation error")
			}
		})
	}
}

func TestDecodeBundlesPreservesHiddenStateAndMultipleOrigins(t *testing.T) {
	t.Parallel()

	blogs, graph := testBundleJSON()
	bundles, err := DecodeBundles(blogs, graph)
	if err != nil {
		t.Fatalf("DecodeBundles() error = %v", err)
	}
	plan, err := BuildPlan(bundles, sequenceGenerator("AAAAAAAAA", "BBBBBBBBB"))
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if plan.Sites[1].Visibility != "HIDDEN" || plan.Sites[1].VisibilityReason != "FRIEND_LINK_DISCOVERY_PENDING_REVIEW" {
		t.Fatalf("hidden site = %#v, want preserved review state", plan.Sites[1])
	}
	if len(plan.Origins) != 4 {
		t.Fatalf("origins = %#v, want all four per-site provenance rows", plan.Origins)
	}
}

func TestBuildPlanRejectsDuplicateNormalizedSiteResources(t *testing.T) {
	t.Parallel()

	blogs, friends := testBundleJSON()
	blogs = []byte(strings.Replace(string(blogs), `"link_page": "https://links.example/page"`, `"link_page": "/sitemap.xml"`, 1))
	bundles, err := DecodeBundles(blogs, friends)
	if err != nil {
		t.Fatalf("DecodeBundles() error = %v", err)
	}
	if _, err := BuildPlan(bundles, sequenceGenerator("AAAAAAAAA", "BBBBBBBBB")); err == nil {
		t.Fatal("BuildPlan() error = nil, want duplicate normalized resource rejected before transaction")
	}
}

func TestBuildPlanRejectsJoinedBeforeCreated(t *testing.T) {
	t.Parallel()

	blogs, graph := testBundleJSON()
	bundles, err := DecodeBundles(blogs, graph)
	if err != nil {
		t.Fatalf("DecodeBundles() error = %v", err)
	}
	bundles.Blogs.Blogs[0].CreatedAt = "2025-01-02T00:00:00Z"
	if _, err := BuildPlan(bundles, sequenceGenerator("AAAAAAAAA", "BBBBBBBBB")); err == nil {
		t.Fatal("BuildPlan() error = nil, want joined_at before created_at rejected")
	}
}

func TestBuildPlanRejectsInvalidSoftwareComponentURLsBeforeTransaction(t *testing.T) {
	t.Parallel()

	blogs, friends := testBundleJSON()
	bundles, err := DecodeBundles(blogs, friends)
	if err != nil {
		t.Fatalf("DecodeBundles() error = %v", err)
	}
	for _, invalidURL := range []string{"ftp://packages.example/astro", "HTTPS://packages.example/astro"} {
		bundles.Blogs.Blogs[0].Architecture.Program.WebsiteURL = &invalidURL
		if _, err := BuildPlan(bundles, sequenceGenerator("AAAAAAAAA", "BBBBBBBBB")); err == nil {
			t.Fatalf("BuildPlan() error = nil for %q, want invalid component URL rejected before transaction", invalidURL)
		}
	}
}

func TestBuildPlanMapsDirectoryRelationsAndRetriesShortIDCollision(t *testing.T) {
	t.Parallel()

	blogs, friends := testBundleJSON()
	bundles, err := DecodeBundles(blogs, friends)
	if err != nil {
		t.Fatalf("DecodeBundles() error = %v", err)
	}
	generated := []string{"AAAAAAAAA", "AAAAAAAAA", "BBBBBBBBB"}
	generateCalls := 0
	plan, err := BuildPlan(bundles, func() (string, error) {
		value := generated[generateCalls]
		generateCalls++
		return value, nil
	})
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}

	if generateCalls != 3 || plan.Sites[0].ShortID != "AAAAAAAAA" || plan.Sites[1].ShortID != "BBBBBBBBB" {
		t.Fatalf("short IDs = (%d, %q, %q), want collision retry", generateCalls, plan.Sites[0].ShortID, plan.Sites[1].ShortID)
	}
	first := plan.Sites[0]
	if first.ID != testSiteIDOne || first.Scheme != "https" || first.NormalizedHost != "example.com" || first.BasePath != "/blog" {
		t.Fatalf("first site address = %#v, want normalized imported address", first)
	}
	if first.AccessScope != "GLOBAL_ONLY" || first.Visibility != "VISIBLE" || first.Summary != "Example summary" {
		t.Fatalf("first site profile = %#v, want mapped legacy profile", first)
	}
	if !first.JoinedAt.Equal(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)) || !first.CreatedAt.Equal(first.JoinedAt) {
		t.Fatalf("site times = (%s, %s), want join time preserved as created time", first.JoinedAt, first.CreatedAt)
	}
	if len(plan.Feeds) != 1 || plan.Feeds[0].LocationType != "RELATIVE" || plan.Feeds[0].URLRef != "/blog/feed.xml" || plan.Feeds[0].Format != "ATOM" {
		t.Fatalf("feeds = %#v, want normalized relative Atom feed", plan.Feeds)
	}
	if len(plan.Resources) != 2 || plan.Resources[0].Kind != "LINK_PAGE" || plan.Resources[1].Kind != "SITEMAP" {
		t.Fatalf("resources = %#v, want deterministic link-page and sitemap rows", plan.Resources)
	}
	if len(plan.Tags) != 2 || plan.Tags[1].Slug != "legacy-"+testTagIDSub {
		t.Fatalf("tags = %#v, want machine-key and deterministic legacy slug", plan.Tags)
	}
	if len(plan.SiteTags) != 2 || plan.SiteTags[0].TopicRole != "PRIMARY" || plan.SiteTags[1].TopicRole != "SECONDARY" {
		t.Fatalf("site tags = %#v, want imported primary and secondary roles", plan.SiteTags)
	}
	if len(plan.Components) != 2 || plan.Components[0].ID != testProgramID || plan.Components[1].ID != testStackID {
		t.Fatalf("components = %#v, want program and stack IDs preserved", plan.Components)
	}
	if len(plan.Dependencies) != 1 || plan.Dependencies[0].Role != "LANGUAGE" {
		t.Fatalf("dependencies = %#v, want typed program dependency", plan.Dependencies)
	}
	if len(plan.SiteComponents) != 1 || plan.SiteComponents[0].Role != "SITE_PROGRAM" {
		t.Fatalf("site components = %#v, want imported site program", plan.SiteComponents)
	}
	if len(plan.Sources) != 4 || len(plan.Origins) != 4 || len(plan.Origins[0].Metadata) == 0 {
		t.Fatalf("provenance = (%#v, %#v), want four source types and metadata-preserving origins", plan.Sources, plan.Origins)
	}
	if len(plan.FriendLinks) != 1 || plan.FriendLinks[0].SourceSiteID != testSiteIDOne || plan.FriendLinks[0].TargetHost != "other.example" {
		t.Fatalf("friend links = %#v, want both endpoints resolved to registered sites", plan.FriendLinks)
	}
	if got := plan.Counts(); got != (Counts{Sites: 2, Feeds: 1, Resources: 2, Tags: 2, SiteTags: 2, SoftwareComponents: 2, Dependencies: 1, SiteComponents: 1, Sources: 4, Origins: 4, FriendLinks: 1}) {
		t.Fatalf("Counts() = %#v, want mapped row totals", got)
	}
}

func TestServiceRejectsConcurrentImportBeforeSecondStoreCall(t *testing.T) {
	t.Parallel()

	blogs, friends := testBundleJSON()
	bundles, err := DecodeBundles(blogs, friends)
	if err != nil {
		t.Fatalf("DecodeBundles() error = %v", err)
	}
	store := &blockingStore{started: make(chan struct{}), release: make(chan struct{})}
	shortIDs := []string{"AAAAAAAAA", "BBBBBBBBB"}
	shortIDIndex := 0
	service := NewService(store, func() (string, error) {
		value := shortIDs[shortIDIndex]
		shortIDIndex++
		return value, nil
	})
	firstDone := make(chan error, 1)
	go func() {
		_, importErr := service.Import(context.Background(), bundles)
		firstDone <- importErr
	}()
	<-store.started

	if _, err := service.Import(context.Background(), bundles); !errors.Is(err, ErrImportRunning) {
		t.Fatalf("second Import() error = %v, want ErrImportRunning", err)
	}
	close(store.release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first Import() error = %v", err)
	}
	if store.calls != 1 {
		t.Fatalf("store calls = %d, want 1", store.calls)
	}
}

type blockingStore struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
	calls   int
}

func sequenceGenerator(values ...string) func() (string, error) {
	index := 0
	return func() (string, error) {
		value := values[index%len(values)]
		index++
		return value, nil
	}
}

func (store *blockingStore) Import(_ context.Context, plan Plan) (Counts, error) {
	store.calls++
	store.once.Do(func() { close(store.started) })
	<-store.release
	return plan.Counts(), nil
}
