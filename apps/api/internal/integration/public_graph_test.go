//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"testing"

	"heyblog-api/internal/features/publicview"
	dbgen "heyblog-api/internal/infrastructure/database/gen"

	"github.com/jackc/pgx/v5/pgxpool"
)

func verifyPublicFriendGraph(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	// Given
	queries := dbgen.New(pool)
	source := insertSite(ctx, t, pool, "G1a2B3c4D", "Graph Source", "graph-source.example.com")
	target := insertSite(ctx, t, pool, "G2a3B4c5D", "Graph Target", "graph-target.example.com")
	insertSite(ctx, t, pool, "G3a4B5c6D", "Graph Isolated", "graph-isolated.example.com")
	for _, link := range []dbgen.UpsertFriendLinkParams{
		{PSourceSiteID: source, PTargetUrl: "https://graph-target.example.com/", PTargetHost: "graph-target.example.com", PStatus: "ACTIVE"},
		{PSourceSiteID: target, PTargetUrl: "https://graph-source.example.com/", PTargetHost: "graph-source.example.com", PStatus: "ACTIVE"},
		{PSourceSiteID: source, PTargetUrl: "https://graph-external.example.com/blog", PTargetHost: "graph-external.example.com", PStatus: "ACTIVE"},
		{PSourceSiteID: target, PTargetUrl: "https://graph-inactive.example.com/", PTargetHost: "graph-inactive.example.com", PStatus: "INACTIVE"},
	} {
		if err := queries.UpsertFriendLink(ctx, link); err != nil {
			t.Fatal(err)
		}
	}
	service := publicview.New(queries, nil)
	// When
	graph, err := service.SiteGraphByIdentifier(ctx, publicview.SiteIdentifier{Kind: publicview.IdentifierShortID, Value: "G1a2B3c4D"})
	// Then
	if err != nil || len(graph.Nodes) != 3 || len(graph.Edges) != 3 || graph.Stats.ReciprocalPairs != 1 {
		t.Fatalf("local graph = %#v, error = %v", graph, err)
	}
	t.Run("global includes isolated and excludes inactive", func(t *testing.T) {
		graph, err := service.Graph(ctx)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, node := range graph.Nodes {
			if node.Host == "graph-isolated.example.com" {
				found = true
			}
			if node.Host == "graph-inactive.example.com" {
				t.Fatal("inactive external exposed")
			}
		}
		if !found {
			t.Fatal("isolated site missing")
		}
	})
	t.Run("external promotion updates identity and address", func(t *testing.T) {
		insertSite(ctx, t, pool, "G4a5B6c7D", "Promoted", "graph-external.example.com")
		graph, err := service.SiteGraphByIdentifier(ctx, publicview.SiteIdentifier{Kind: publicview.IdentifierShortID, Value: "G1a2B3c4D"})
		if err != nil {
			t.Fatal(err)
		}
		for _, node := range graph.Nodes {
			if node.Host == "graph-external.example.com" && (node.ID != "site:G4a5B6c7D" || node.HomepageURL != "https://graph-external.example.com/") {
				t.Fatalf("promoted node = %#v", node)
			}
		}
	})
	t.Run("hidden center and endpoint", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "UPDATE directory.sites SET visibility = 'HIDDEN', visibility_reason = 'private' WHERE id = $1", source); err != nil {
			t.Fatal(err)
		}
		graph, err := service.SiteGraphByIdentifier(ctx, publicview.SiteIdentifier{Kind: publicview.IdentifierShortID, Value: "G1a2B3c4D"})
		if err != nil || len(graph.Nodes) != 0 || len(graph.Edges) != 0 {
			t.Fatalf("hidden local = %#v, error = %v", graph, err)
		}
		data, err := queries.GetPublicFriendGraph(ctx, target)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &graph); err != nil {
			t.Fatal(err)
		}
		if len(graph.Nodes) != 1 || len(graph.Edges) != 0 {
			t.Fatalf("hidden endpoint = %#v", graph)
		}
	})
	t.Run("moved address preserves graph identity", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "UPDATE directory.sites SET scheme = 'http', normalized_host = 'graph-moved.example.com', base_path = '/blog' WHERE id = $1", target); err != nil {
			t.Fatal(err)
		}
		graph, err := service.SiteGraphByIdentifier(ctx, publicview.SiteIdentifier{Kind: publicview.IdentifierShortID, Value: "G2a3B4c5D"})
		if err != nil || len(graph.Nodes) != 1 || graph.Nodes[0].ID != "site:G2a3B4c5D" || graph.Nodes[0].HomepageURL != "http://graph-moved.example.com/blog" {
			t.Fatalf("moved graph = %#v, error = %v", graph, err)
		}
	})
	t.Run("removed and missing centers return not found", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "UPDATE directory.sites SET visibility = 'REMOVED', visibility_reason = 'removed' WHERE id = $1", source); err != nil {
			t.Fatal(err)
		}
		for _, shortID := range []string{"G1a2B3c4D", "Z9z8Y7y6X"} {
			if _, err := service.SiteGraphByIdentifier(ctx, publicview.SiteIdentifier{Kind: publicview.IdentifierShortID, Value: shortID}); err == nil {
				t.Fatalf("%s graph unexpectedly succeeded", shortID)
			}
		}
	})
}
