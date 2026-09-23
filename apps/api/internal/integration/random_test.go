//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "heyblog-api/internal/database/gen"
)

// Leave each child as the only visible candidate in turn, avoiding probabilistic assertions.
func verifyRandomClassificationSelection(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	cascades, err := queries.ListEnabledSiteTagCascades(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var pair []dbgen.ListEnabledSiteTagCascadesRow
	for _, first := range cascades {
		for _, second := range cascades {
			if first.Level1ID == second.Level1ID && first.Level2ID != second.Level2ID {
				pair = []dbgen.ListEnabledSiteTagCascadesRow{first, second}
				break
			}
		}
		if len(pair) == 2 {
			break
		}
	}
	if len(pair) != 2 {
		t.Fatal("expected a seeded parent with two children")
	}
	if _, err := tx.Exec(ctx, `UPDATE directory.sites SET visibility = 'HIDDEN', visibility_reason = 'random fixture' WHERE visibility = 'VISIBLE'`); err != nil {
		t.Fatal(err)
	}
	var siteID pgtype.UUID
	if err := tx.QueryRow(ctx, `INSERT INTO directory.sites (short_id, name, normalized_host, tag_cascade_id) VALUES ('Rnd000001', 'Random Fixture', 'random-fixture.example.com', $1) RETURNING id`, pair[0].ID).Scan(&siteID); err != nil {
		t.Fatal(err)
	}
	for _, cascade := range pair {
		if _, err := tx.Exec(ctx, `UPDATE directory.sites SET tag_cascade_id = $2 WHERE id = $1`, siteID, cascade.ID); err != nil {
			t.Fatal(err)
		}
		selected, err := queries.PickRandomVisibleSite(ctx, dbgen.PickRandomVisibleSiteParams{Level1TagName: cascade.Level1Name})
		if err != nil || selected.ID != siteID || selected.TagCascadeID != cascade.ID {
			t.Fatalf("parent-only selection for child %q = (%#v, %v)", cascade.Level2Name, selected, err)
		}
	}
}
