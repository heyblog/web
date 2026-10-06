//go:build integration

package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"heyblog-api/internal/features/publicview"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func verifySitemapQueries(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	// Given isolated runtime-role fixtures, rolled back before other infrastructure scenarios.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin sitemap fixtures: %v", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("rollback sitemap fixtures: %v", err)
		}
	}()
	if _, err := tx.Exec(ctx, `
		INSERT INTO directory.sites (id, short_id, name, normalized_host)
		SELECT ('00000000-0000-7000-8000-' || lpad(number::text, 12, '0'))::uuid,
		       lpad(number::text, 9, '0'), 'Sitemap site', 'sitemap-' || number || '.example.test'
		  FROM generate_series(1, 1002) AS number;
	`); err != nil {
		t.Fatalf("insert sitemap sites: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO directory.sites (id, short_id, name, normalized_host, visibility, visibility_reason)
		VALUES ('00000000-0000-7000-8000-000000002001', '000002001', 'Hidden', 'sitemap-hidden.example.test', 'HIDDEN', 'internal'),
		       ('00000000-0000-7000-8000-000000002002', '000002002', 'Removed', 'sitemap-removed.example.test', 'REMOVED', 'internal');
	`); err != nil {
		t.Fatalf("insert excluded sitemap sites: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	start := now.Add(-2 * time.Hour)
	published := now.Add(-3 * time.Hour)
	updated := now.Add(-time.Hour)
	if _, err := tx.Exec(ctx, `
		INSERT INTO content.announcements (id, kind, title, status, starts_at, ends_at, published_at, updated_at)
		SELECT ('00000000-0000-7000-8000-' || lpad(number::text, 12, '0'))::uuid,
		       'MAIN', 'Expired sitemap announcement', 'PUBLISHED', $1, $3, $2, $3
		  FROM generate_series(1, 1002) AS number;
	`, start, published, updated); err != nil {
		t.Fatalf("insert expired sitemap announcements: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO content.announcements (id, kind, title, status, starts_at, published_at, archived_at, updated_at)
		VALUES ('00000000-0000-7000-8000-000000002001', 'MAIN', 'Active', 'PUBLISHED', $1::timestamptz, $2::timestamptz, NULL, $3::timestamptz),
		       ('00000000-0000-7000-8000-000000002002', 'MAIN', 'Archived after start', 'ARCHIVED', $1, $2, $3, $3),
		       ('00000000-0000-7000-8000-000000002003', 'MAIN', 'Archived before start', 'ARCHIVED', $1, $2, $1 - interval '1 minute', $3),
		       ('00000000-0000-7000-8000-000000002004', 'MAIN', 'Archived exactly at start', 'ARCHIVED', $1, $2, $1, $3),
		       ('00000000-0000-7000-8000-000000002005', 'MAIN', 'Scheduled', 'PUBLISHED', $3 + interval '2 hours', $2, NULL, $3),
		       ('00000000-0000-7000-8000-000000002006', 'MAIN', 'Draft', 'DRAFT', NULL, NULL, NULL, $3),
		       ('00000000-0000-7000-8000-000000002007', 'BANNER', 'Banner', 'PUBLISHED', $1, $2, NULL, $3);
	`, start, published, updated); err != nil {
		t.Fatalf("insert announcement visibility edges: %v", err)
	}
	queries := dbgen.New(tx)
	service := publicview.New(queries, nil)
	siteRows, err := queries.ListSitemapSites(ctx, pgtype.UUID{})
	if err != nil || len(siteRows) != 1001 {
		t.Fatalf("site query lookahead count = %d, %v", len(siteRows), err)
	}
	announcementRows, err := queries.ListSitemapAnnouncements(ctx, pgtype.UUID{})
	if err != nil || len(announcementRows) != 1001 {
		t.Fatalf("announcement query lookahead count = %d, %v", len(announcementRows), err)
	}
	for _, scenario := range []struct {
		kind  publicview.SitemapKind
		count int
	}{
		{kind: publicview.SitemapSites, count: 1002},
		{kind: publicview.SitemapAnnouncements, count: 1004},
	} {
		// When all public sitemap pages are requested in keyset order.
		first, err := service.Sitemap(ctx, publicview.SitemapQuery{Kind: scenario.kind})
		if err != nil {
			t.Fatalf("first %s page: %v", scenario.kind, err)
		}
		// Then lookahead creates a cursor at the last emitted UUID, not at the skipped row.
		if len(first.Items) != 1000 || first.NextAfter == nil || *first.NextAfter != "00000000-0000-7000-8000-000000001000" {
			t.Fatalf("first %s page length/cursor = %d/%v", scenario.kind, len(first.Items), first.NextAfter)
		}
		var cursor pgtype.UUID
		if err := cursor.Scan(*first.NextAfter); err != nil {
			t.Fatal(err)
		}
		second, err := service.Sitemap(ctx, publicview.SitemapQuery{Kind: scenario.kind, After: cursor})
		if err != nil {
			t.Fatalf("second %s page: %v", scenario.kind, err)
		}
		if len(second.Items) != scenario.count-1000 || second.NextAfter != nil || second.Items[0].ID != "00000000-0000-7000-8000-000000001001" {
			t.Fatalf("second %s page = %#v", scenario.kind, second)
		}
		previous := ""
		for _, item := range append(first.Items, second.Items...) {
			if item.ID <= previous {
				t.Fatalf("%s IDs are duplicated or unordered", scenario.kind)
			}
			previous = item.ID
			if scenario.kind == publicview.SitemapAnnouncements {
				if item.StartsAt == nil || item.PublishedAt == nil || item.UpdatedAt == nil || !item.StartsAt.Equal(start) || !item.PublishedAt.Equal(published) || !item.UpdatedAt.Equal(updated) {
					t.Fatalf("announcement timestamps = %#v", item)
				}
			}
		}
	}
	verifySitemapDetailAgreement(ctx, t, queries, service, published, updated)
}

func verifySitemapDetailAgreement(ctx context.Context, t *testing.T, queries *dbgen.Queries, service *publicview.Service, published, updated time.Time) {
	t.Helper()
	// Given announcement boundary fixtures and the same public detail predicates.
	for _, scenario := range []struct {
		id      string
		isFound bool
	}{
		{id: "00000000-0000-7000-8000-000000002001", isFound: true},
		{id: "00000000-0000-7000-8000-000000002002", isFound: true},
		{id: "00000000-0000-7000-8000-000000002003"},
		{id: "00000000-0000-7000-8000-000000002004"},
		{id: "00000000-0000-7000-8000-000000002005"},
		{id: "00000000-0000-7000-8000-000000002006"},
		{id: "00000000-0000-7000-8000-000000002007"},
	} {
		var id pgtype.UUID
		if err := id.Scan(scenario.id); err != nil {
			t.Fatal(err)
		}
		// When the authoritative detail query resolves the same UUID.
		_, err := queries.GetPublicAnnouncementByID(ctx, id)
		// Then detail and sitemap inclusion agree, including the exact archive-start boundary.
		if scenario.isFound && err != nil || !scenario.isFound && !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("detail %s inclusion mismatch: %v", scenario.id, err)
		}
	}
	// When a public announcement DTO is returned.
	view, err := service.AnnouncementByID(ctx, "00000000-0000-7000-8000-000000002002")
	// Then stored publication/update times survive into the DTO.
	if err != nil || !view.PublishedAt.Equal(published) || !view.UpdatedAt.Equal(updated) {
		t.Fatalf("public announcement timestamps = %#v, %v", view, err)
	}
	// When a hidden site remains accessible through its existing detail route.
	hidden, err := service.SiteByIdentifier(ctx, publicview.SiteIdentifier{Kind: publicview.IdentifierShortID, Value: "000002001"})
	// Then it explicitly opts out of indexing without changing access behavior.
	if err != nil || hidden.IsIndexable {
		t.Fatalf("hidden profile indexability = %v, %v", hidden.IsIndexable, err)
	}
}
