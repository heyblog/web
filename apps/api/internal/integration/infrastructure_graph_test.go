//go:build integration

package integration_test

import (
	"context"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func verifySoftwareComponentDependencies(ctx context.Context, t *testing.T, connection *pgxpool.Pool) {
	t.Helper()
	queries := dbgen.New(connection)

	softwareA := createSoftwareComponent(ctx, t, queries, "Software A", "software a")
	astro := createSoftwareComponent(ctx, t, queries, "Astro", "astro")
	nodejs := createSoftwareComponent(ctx, t, queries, "Node.js", "node.js")

	for _, dependency := range []dbgen.AddSoftwareComponentDependencyParams{
		{ComponentID: softwareA.ID, DependencyComponentID: astro.ID, Role: "FRAMEWORK"},
		{ComponentID: softwareA.ID, DependencyComponentID: nodejs.ID, Role: "RUNTIME"},
	} {
		if _, err := queries.AddSoftwareComponentDependency(ctx, dependency); err != nil {
			t.Fatalf("add software component dependency: %v", err)
		}
	}

	dependencies, err := queries.ListSoftwareComponentDependencies(ctx, softwareA.ID)
	if err != nil {
		t.Fatalf("list software component dependencies: %v", err)
	}
	if len(dependencies) != 2 ||
		dependencies[0].Name != "Astro" || dependencies[0].Role != "FRAMEWORK" ||
		dependencies[1].Name != "Node.js" || dependencies[1].Role != "RUNTIME" {
		t.Fatalf("software component dependencies = %#v", dependencies)
	}

	if _, err := queries.AddSoftwareComponentDependency(ctx, dbgen.AddSoftwareComponentDependencyParams{
		ComponentID:           softwareA.ID,
		DependencyComponentID: softwareA.ID,
		Role:                  "OTHER",
	}); err == nil {
		t.Fatal("self software component dependency unexpectedly succeeded")
	}

	transaction, err := connection.Begin(ctx)
	if err != nil {
		t.Fatalf("begin dependency cycle transaction: %v", err)
	}
	_, cycleErr := transaction.Exec(ctx, `
		INSERT INTO directory.software_component_dependencies (
			component_id, dependency_component_id, role
		) VALUES ($1, $2, 'OTHER')
	`, astro.ID, softwareA.ID)
	if err := transaction.Rollback(ctx); err != nil {
		t.Fatalf("rollback dependency cycle transaction: %v", err)
	}
	if cycleErr == nil {
		t.Fatal("indirect software component dependency cycle unexpectedly succeeded")
	}

	if _, err := connection.Exec(ctx, "DELETE FROM directory.software_components WHERE id = $1", astro.ID); err == nil {
		t.Fatal("referenced dependency component deletion unexpectedly succeeded")
	}
	if _, err := connection.Exec(ctx, "DELETE FROM directory.software_components WHERE id = $1", softwareA.ID); err != nil {
		t.Fatalf("delete component owning dependency relations: %v", err)
	}

	var dependencyCount int
	if err := connection.QueryRow(ctx, `
		SELECT count(*)
		  FROM directory.software_component_dependencies
		 WHERE component_id = $1
	`, softwareA.ID).Scan(&dependencyCount); err != nil {
		t.Fatalf("count deleted component dependencies: %v", err)
	}
	if dependencyCount != 0 {
		t.Fatalf("deleted component dependency count = %d, want 0", dependencyCount)
	}
}

func createSoftwareComponent(
	ctx context.Context,
	t *testing.T,
	queries *dbgen.Queries,
	name string,
	normalizedName string,
) dbgen.DirectorySoftwareComponent {
	t.Helper()
	component, err := queries.CreateSoftwareComponent(ctx, dbgen.CreateSoftwareComponentParams{
		Name:           name,
		NormalizedName: normalizedName,
		Description:    "",
		IsOpenSource:   true,
	})
	if err != nil {
		t.Fatalf("create software component %q: %v", name, err)
	}
	return component
}

func verifyFriendLinkGraph(
	ctx context.Context,
	t *testing.T,
	connection *pgxpool.Pool,
	adminConnection *pgx.Conn,
) {
	t.Helper()
	queries := dbgen.New(connection)

	sourceID := insertSite(ctx, t, connection, "4Aa5Bb6Cc", "Source", "source.example.com")
	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.site_feeds (
			site_id, name, location_type, url_ref, url_key, is_default
		) VALUES ($1, 'Default', 'RELATIVE', '/feed.xml', '/feed.xml', true)
	`, sourceID); err != nil {
		t.Fatalf("insert source feed: %v", err)
	}

	externalURL := "https://target.example.com/friends"
	if err := queries.UpsertFriendLink(ctx, dbgen.UpsertFriendLinkParams{
		PSourceSiteID: sourceID,
		PTargetUrl:    externalURL,
		PTargetHost:   "target.example.com",
		PStatus:       "ACTIVE",
	}); err != nil {
		t.Fatalf("upsert external friend link: %v", err)
	}

	links := listFriendLinks(ctx, t, queries, sourceID, false)
	if len(links) != 1 || links[0].TargetSiteID.Valid || links[0].TargetUrl != externalURL {
		t.Fatalf("external friend links = %#v", links)
	}
	if err := queries.UpsertFriendLink(ctx, dbgen.UpsertFriendLinkParams{
		PSourceSiteID: sourceID,
		PTargetUrl:    externalURL + "?from=source",
		PTargetHost:   "target.example.com",
		PStatus:       "ACTIVE",
	}); err == nil {
		t.Fatal("friend-link URL with query unexpectedly succeeded")
	}
	if _, err := connection.Exec(ctx, `
		UPDATE directory.sites
		   SET visibility = 'HIDDEN', visibility_reason = 'private'
		 WHERE id = $1
	`, sourceID); err != nil {
		t.Fatalf("hide source site: %v", err)
	}
	if links = listFriendLinks(ctx, t, queries, sourceID, true); len(links) != 0 {
		t.Fatalf("hidden source friend links remained public: %#v", links)
	}
	if _, err := connection.Exec(ctx, `
		UPDATE directory.sites
		   SET visibility = 'VISIBLE', visibility_reason = NULL
		 WHERE id = $1
	`, sourceID); err != nil {
		t.Fatalf("show source site: %v", err)
	}

	targetID := insertSite(ctx, t, connection, "5Aa6Bb7Cc", "Target", "target.example.com")
	links = listFriendLinks(ctx, t, queries, sourceID, false)
	if len(links) != 1 || !links[0].TargetSiteID.Valid || links[0].TargetSiteID != targetID ||
		links[0].TargetUrl != "https://target.example.com/" {
		t.Fatalf("promoted friend links = %#v", links)
	}
	if err := queries.UpsertFriendLink(ctx, dbgen.UpsertFriendLinkParams{
		PSourceSiteID: sourceID,
		PTargetUrl:    "https://target.example.com/friends",
		PTargetHost:   "target.example.com",
		PStatus:       "ACTIVE",
	}); err == nil {
		t.Fatal("registered friend-link URL differing from the site address unexpectedly succeeded")
	}
	if err := queries.UpsertFriendLink(ctx, dbgen.UpsertFriendLinkParams{
		PSourceSiteID: sourceID,
		PTargetUrl:    "https://target.example.com/",
		PTargetHost:   "target.example.com",
		PStatus:       "INACTIVE",
	}); err != nil {
		t.Fatalf("deactivate friend link: %v", err)
	}
	if links = listFriendLinks(ctx, t, queries, sourceID, false); len(links) != 0 {
		t.Fatalf("inactive friend links remained active: %#v", links)
	}
	links = listFriendLinks(ctx, t, queries, sourceID, true)
	if len(links) != 1 || links[0].LinkStatus != "INACTIVE" {
		t.Fatalf("inactive friend links = %#v", links)
	}
	if err := queries.UpsertFriendLink(ctx, dbgen.UpsertFriendLinkParams{
		PSourceSiteID: sourceID,
		PTargetUrl:    "https://target.example.com/",
		PTargetHost:   "target.example.com",
		PStatus:       "ACTIVE",
	}); err != nil {
		t.Fatalf("reactivate friend link: %v", err)
	}
	if _, err := adminConnection.Exec(ctx, "LOAD 'age'"); err != nil {
		t.Fatalf("load AGE in admin session: %v", err)
	}
	futureEvent := time.Now().Add(time.Hour).UnixMilli()
	for _, event := range []struct {
		status      string
		updatedAtMS int64
	}{
		{status: "INACTIVE", updatedAtMS: futureEvent},
		{status: "ACTIVE", updatedAtMS: futureEvent - 1},
	} {
		if _, err := adminConnection.Exec(ctx, `
			SELECT directory.merge_friend_link_graph($1, $2, $3, $4, $5, $6)
		`, sourceID, "target.example.com", "https://target.example.com/", event.status, int64(1), event.updatedAtMS); err != nil {
			t.Fatalf("merge timestamped friend link: %v", err)
		}
	}
	links = listFriendLinks(ctx, t, queries, sourceID, true)
	if len(links) != 1 || links[0].LinkStatus != "INACTIVE" || links[0].CreatedAtMs != 1 ||
		links[0].UpdatedAtMs != futureEvent {
		t.Fatalf("latest timestamp friend links = %#v", links)
	}
	if _, err := adminConnection.Exec(ctx, `
		SELECT directory.merge_friend_link_graph($1, $2, $3, 'ACTIVE', $4, $5)
	`, sourceID, "target.example.com", "https://target.example.com/", int64(2), futureEvent); err != nil {
		t.Fatalf("merge equal timestamp friend link: %v", err)
	}
	links = listFriendLinks(ctx, t, queries, sourceID, false)
	if len(links) != 1 || links[0].LinkStatus != "ACTIVE" || links[0].CreatedAtMs != 1 ||
		links[0].UpdatedAtMs != futureEvent {
		t.Fatalf("equal timestamp friend links = %#v", links)
	}

	if err := queries.UpsertFriendLink(ctx, dbgen.UpsertFriendLinkParams{
		PSourceSiteID: targetID,
		PTargetUrl:    "https://source.example.com/",
		PTargetHost:   "source.example.com",
		PStatus:       "ACTIVE",
	}); err != nil {
		t.Fatalf("upsert reciprocal friend link: %v", err)
	}
	links = listFriendLinks(ctx, t, queries, sourceID, false)
	if len(links) != 1 || !links[0].IsReciprocal {
		t.Fatalf("reciprocal friend links = %#v", links)
	}

	if _, err := connection.Exec(ctx, `
		UPDATE directory.sites
		   SET visibility = 'HIDDEN', visibility_reason = 'private'
		 WHERE id = $1
	`, targetID); err != nil {
		t.Fatalf("hide target site: %v", err)
	}
	if links = listFriendLinks(ctx, t, queries, sourceID, false); len(links) != 0 {
		t.Fatalf("hidden registered target remained public: %#v", links)
	}

	if _, err := connection.Exec(ctx, `
		UPDATE directory.sites
		   SET visibility = 'VISIBLE', visibility_reason = NULL,
		       scheme = 'http', normalized_host = 'moved.example.com', base_path = '/blog'
		 WHERE id = $1
	`, targetID); err != nil {
		t.Fatalf("move target site: %v", err)
	}
	links = listFriendLinks(ctx, t, queries, sourceID, false)
	if len(links) != 1 || links[0].TargetHost != "moved.example.com" ||
		links[0].TargetUrl != "http://moved.example.com/blog" {
		t.Fatalf("moved friend links = %#v", links)
	}

	if err := queries.UpsertFriendLink(ctx, dbgen.UpsertFriendLinkParams{
		PSourceSiteID: sourceID,
		PTargetUrl:    "https://source.example.com/",
		PTargetHost:   "source.example.com",
		PStatus:       "ACTIVE",
	}); err == nil {
		t.Fatal("friend-link self edge unexpectedly succeeded")
	}

	if _, err := connection.Exec(ctx, "DELETE FROM directory.sites WHERE id = $1", targetID); err == nil {
		t.Fatal("hard site deletion unexpectedly succeeded")
	}

	var relativeFeed string
	if err := connection.QueryRow(ctx, `
		SELECT url_ref FROM directory.site_feeds WHERE site_id = $1 AND is_default
	`, sourceID).Scan(&relativeFeed); err != nil {
		t.Fatalf("query relative feed: %v", err)
	}
	if relativeFeed != "/feed.xml" {
		t.Fatalf("relative feed = %q, want /feed.xml", relativeFeed)
	}
}

func listFriendLinks(
	ctx context.Context,
	t *testing.T,
	queries *dbgen.Queries,
	sourceID pgtype.UUID,
	includeInactive bool,
) []dbgen.ListFriendLinksRow {
	t.Helper()
	links, err := queries.ListFriendLinks(ctx, dbgen.ListFriendLinksParams{
		PSourceSiteID:    sourceID,
		PIncludeInactive: includeInactive,
	})
	if err != nil {
		t.Fatalf("list friend links: %v", err)
	}
	return links
}
