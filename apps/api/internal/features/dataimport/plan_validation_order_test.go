package dataimport

import (
	"strings"
	"testing"
)

func TestPlanValidationPreservesFirstFailureAcrossSections(t *testing.T) {
	t.Parallel()
	blogs, graph := testBundleJSON()
	bundles, err := DecodeBundles(blogs, graph)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{"Order0001", "Order0002"}
	plan, err := BuildPlan(bundles, func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Given simultaneous site/feed/resource defects, the earliest section wins.
	plan.Sites[0].Name = ""
	plan.Feeds[0].SiteID = "unknown-feed-site"
	plan.Resources[0].SiteID = "unknown-resource-site"
	if err := validatePlan(plan); err == nil || !strings.Contains(err.Error(), "invalid profile data") {
		t.Fatalf("first failure = %v, want site profile", err)
	}
	plan.Sites[0].Name = "Example"
	if err := validatePlan(plan); err == nil || !strings.Contains(err.Error(), "feed references unknown site") {
		t.Fatalf("first failure = %v, want feed", err)
	}
	plan.Feeds[0].SiteID = plan.Sites[0].ID
	if err := validatePlan(plan); err == nil || !strings.Contains(err.Error(), "resource references unknown site") {
		t.Fatalf("first failure = %v, want resource", err)
	}
}
