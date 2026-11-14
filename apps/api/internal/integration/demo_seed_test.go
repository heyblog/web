//go:build integration

package integration_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"heyblog-api/internal/features/publicview"
	"heyblog-api/internal/infrastructure/database"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func TestDemoSeed(t *testing.T) {
	fixture := newAuditMigrationFixture(t)
	ctx := t.Context()
	adminURL, err := fixture.container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx, databaseURLForRole(t, adminURL, "migrator", "migrator-secret")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../testdata/demo/seed.sql")
	if err != nil {
		t.Fatal(err)
	}
	seed := string(data)
	begin := func(t *testing.T) pgx.Tx {
		t.Helper()
		tx, err := fixture.admin.Begin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
		return tx
	}

	t.Run("complete dataset and public reads", func(t *testing.T) {
		tx := begin(t)
		if _, err := tx.Exec(ctx, seed); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, "SET CONSTRAINTS ALL IMMEDIATE"); err != nil {
			t.Fatalf("deferred dataset constraints: %v", err)
		}
		for _, check := range []struct {
			name  string
			query string
			want  int
		}{
			{"sites", "SELECT count(*) FROM directory.sites", 36},
			{"visible", "SELECT count(*) FROM directory.sites WHERE visibility='VISIBLE'", 32},
			{"hidden", "SELECT count(*) FROM directory.sites WHERE visibility='HIDDEN'", 2},
			{"removed", "SELECT count(*) FROM directory.sites WHERE visibility='REMOVED'", 2},
			{"origins", "SELECT count(*) FROM directory.site_origins", 36},
			{"feeds", "SELECT count(*) FROM directory.site_feeds", 36},
			{"defaults", "SELECT count(*) FROM directory.site_feeds WHERE is_default", 27},
			{"resources", "SELECT count(*) FROM directory.site_resources", 54},
			{"tertiary tags", "SELECT count(*) FROM directory.site_tags WHERE role='TERTIARY'", 36},
			{"warnings", "SELECT count(*) FROM directory.site_tags WHERE role='WARNING'", 5},
			{"programs", "SELECT count(*) FROM directory.site_software_components WHERE role='SITE_PROGRAM'", 36},
			{"metrics", "SELECT count(*) FROM directory.site_metrics WHERE click_count > 0", 36},
			{"graph edges", "SELECT count(*) FROM directory.import_friend_link_pairs()", 68},
		} {
			var got int
			if err := tx.QueryRow(ctx, check.query).Scan(&got); err != nil || got != check.want {
				t.Fatalf("%s = %d, want %d: %v", check.name, got, check.want, err)
			}
		}
		views := publicview.New(dbgen.New(tx), nil)
		graph, err := views.Graph(ctx)
		if err != nil || len(graph.Nodes) != 34 || len(graph.Edges) != 68 || graph.Stats.ReciprocalPairs != 6 {
			t.Fatalf("public graph = %#v: %v", graph, err)
		}
		for _, shortID := range []string{"Demo00031", "Demo00032"} {
			isolated, err := views.SiteGraphByIdentifier(ctx, publicview.SiteIdentifier{
				Kind: publicview.IdentifierShortID, Value: shortID,
			})
			if err != nil || len(isolated.Nodes) != 1 || len(isolated.Edges) != 0 {
				t.Fatalf("isolated graph %s = %#v: %v", shortID, isolated, err)
			}
		}
		if _, err := views.SiteByIdentifier(ctx, publicview.SiteIdentifier{
			Kind: publicview.IdentifierShortID, Value: "Demo00001",
		}); err != nil {
			t.Fatalf("public demo profile: %v", err)
		}
	})

	t.Run("repeat preserves edits and unrelated records", func(t *testing.T) {
		tx := begin(t)
		if _, err := tx.Exec(ctx, seed); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `UPDATE directory.sites SET name='Edited demo' WHERE short_id='Demo00001';
			INSERT INTO directory.sites (short_id, name, normalized_host)
			VALUES ('Other0001', 'Unrelated site', 'unrelated.example.test');
			UPDATE directory.site_metrics SET click_count=9999
			WHERE site_id=(SELECT id FROM directory.sites WHERE short_id='Demo00001')`); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, seed); err != nil {
			t.Fatal(err)
		}
		var preserved bool
		if err := tx.QueryRow(ctx, `SELECT
			(SELECT count(*) FROM directory.sites)=37 AND
			(SELECT count(*) FROM directory.site_origins)=36 AND
			(SELECT count(*) FROM directory.site_feeds)=36 AND
			(SELECT count(*) FROM directory.import_friend_link_pairs())=68 AND
			EXISTS (SELECT 1 FROM directory.sites s JOIN directory.site_metrics m ON m.site_id=s.id
				WHERE s.short_id='Demo00001' AND s.name='Edited demo' AND s.revision=2 AND m.click_count=9999) AND
			EXISTS (SELECT 1 FROM directory.sites WHERE short_id='Other0001' AND name='Unrelated site')`).Scan(&preserved); err != nil || !preserved {
			t.Fatalf("repeat import preserved edits = %v: %v", preserved, err)
		}
	})

	t.Run("collision rolls back catalogs and graph", func(t *testing.T) {
		tx := begin(t)
		if _, err := tx.Exec(ctx, `INSERT INTO directory.sites (short_id, name, normalized_host)
			VALUES ('Other0002', 'Existing host', 'blog-20.heyblog.test')`); err != nil {
			t.Fatal(err)
		}
		const snapshotSQL = `SELECT jsonb_build_object(
			'sites', (SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM directory.sites s),
			'sources', (SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM directory.site_sources s),
			'tags', (SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM directory.tags s),
			'components', (SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM directory.software_components s),
			'graph', directory.public_friend_graph(NULL),
			'graph_revision', (SELECT revision FROM runtime_cache.public_graph_revision WHERE singleton)
		)::text`
		var before, after string
		if err := tx.QueryRow(ctx, snapshotSQL).Scan(&before); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, "SAVEPOINT before_seed"); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, seed); err == nil || !strings.Contains(err.Error(), "sites_normalized_host_unique_idx") {
			t.Fatalf("host collision error = %v", err)
		}
		if _, err := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT before_seed"); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(ctx, snapshotSQL).Scan(&after); err != nil || before != after {
			t.Fatalf("collision changed existing data: %v", err)
		}
	})

	t.Run("source collision refused", func(t *testing.T) {
		tx := begin(t)
		if _, err := tx.Exec(ctx, `INSERT INTO directory.site_sources (source_key, name)
			VALUES ('LOCAL_DEMO', 'Unrelated source')`); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, seed); err == nil || !strings.Contains(err.Error(), "marker conflicts") {
			t.Fatalf("source collision error = %v", err)
		}
	})
}
