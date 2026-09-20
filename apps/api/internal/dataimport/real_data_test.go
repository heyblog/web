//go:build migrationdata

package dataimport

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRealCleanedBundlesBuildCompletePlan(t *testing.T) {
	directory := os.Getenv("HEYBLOG_IMPORT_FIXTURE_DIR")
	if directory == "" {
		t.Skip("HEYBLOG_IMPORT_FIXTURE_DIR is not set")
	}
	blogs, err := os.ReadFile(filepath.Join(directory, "blogs.cleaned.json"))
	if err != nil {
		t.Fatalf("read cleaned blogs: %v", err)
	}
	graph, err := os.ReadFile(filepath.Join(directory, "graph.cleaned.json"))
	if err != nil {
		t.Fatalf("read cleaned graph: %v", err)
	}
	bundles, err := DecodeBundles(blogs, graph)
	if err != nil {
		t.Fatalf("DecodeBundles() error = %v", err)
	}
	shortID := 0
	plan, err := BuildPlan(bundles, func() (string, error) {
		shortID++
		return fmt.Sprintf("%09d", shortID), nil
	})
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	counts := plan.Counts()
	t.Logf("real import counts: %#v", counts)
	if counts.Sites != bundles.Blogs.Count || counts.FriendLinks != bundles.Graph.EdgeCount ||
		counts.Sources != 3 || counts.Origins < counts.Sites {
		t.Fatalf("Counts() = %#v, want sites/edges from bundles, three sources, and at least one origin per site", counts)
	}
}
