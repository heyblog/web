//go:build integration

package integration_test

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"heyblog-api/internal/features/dataimport"
)

func TestTagNamesAndHistoricalAliasConstraints(t *testing.T) {
	fixture := newAuditMigrationFixture(t)
	ctx := t.Context()
	if _, err := fixture.provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	t.Run("fresh seeds contain only current import identities", func(t *testing.T) {
		var tags, paths, aliases, labels, obsolete int
		err := fixture.pool.QueryRow(ctx, `SELECT
			(SELECT count(*) FROM directory.tags),
			(SELECT count(*) FROM directory.tag_cascades),
			(SELECT count(*) FROM directory.tag_identity_aliases WHERE alias_kind='TAG' AND system_key IS NOT NULL),
			(SELECT count(*) FROM directory.tag_identity_aliases WHERE alias_kind='LABEL'),
			(SELECT count(*) FROM directory.tag_identity_aliases a JOIN directory.tags t ON t.id=a.tag_id
			 WHERE a.snapshot <> jsonb_build_object('id',a.alias_id::text,'name',t.name))`).Scan(&tags, &paths, &aliases, &labels, &obsolete)
		if err != nil || tags != 71 || paths != 142 || aliases != 81 || labels != 0 || obsolete != 0 {
			t.Fatalf("initial tags=%d paths=%d import identities=%d labels=%d obsolete snapshots=%d: %v", tags, paths, aliases, labels, obsolete, err)
		}
	})
	if _, err := fixture.pool.Exec(ctx, `INSERT INTO directory.tags(name) VALUES('First name')`); err != nil {
		t.Fatal(err)
	}
	t.Run("case uniqueness", func(t *testing.T) {
		_, err := fixture.pool.Exec(ctx, `INSERT INTO directory.tags(name) VALUES('  FIRST NAME  ')`)
		var pgerr *pgconn.PgError
		if !errors.As(err, &pgerr) || pgerr.Code != "23505" {
			t.Fatalf("duplicate normalized name error = %v", err)
		}
	})
	t.Run("LABEL cannot carry system key", func(t *testing.T) {
		if _, err := fixture.pool.Exec(ctx, `INSERT INTO directory.tag_identity_aliases(alias_kind,alias_id,tag_id,snapshot)
			SELECT 'LABEL',uuidv7(),id,jsonb_build_object('name',name) FROM directory.tags WHERE name='First name'`); err != nil {
			t.Fatal(err)
		}
		_, err := fixture.pool.Exec(ctx, `UPDATE directory.tag_identity_aliases SET system_key='invalid-label-key' WHERE alias_kind='LABEL' `)
		var pgerr *pgconn.PgError
		if !errors.As(err, &pgerr) || pgerr.Code != "23514" {
			t.Fatalf("LABEL system key error = %v", err)
		}
	})
	t.Run("SQLite import keys resolve current tags", func(t *testing.T) {
		siteID := insertSite(ctx, t, fixture.pool, "ClnS12345", "Clean seeds", "clean-seeds.example.test")
		_, err := dataimport.NewRepository(fixture.pool).ImportTaxonomy(ctx, dataimport.TagTaxonomyBundle{
			Tags: []dataimport.TagTaxonomyDefinition{{Source: "SQLITE", TagID: "computer", Name: "计算机"}},
			Sites: []dataimport.SiteTagTaxonomyMigration{{
				SiteID: integrationUUIDText(t, siteID), CascadeKey: "other/topic-other-other",
				TertiaryTags: []dataimport.TagTaxonomyTertiary{{Source: "SQLITE", TagID: "computer", Position: 1}},
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		var name string
		if err := fixture.pool.QueryRow(ctx, `SELECT t.name FROM directory.site_tags a JOIN directory.tags t ON t.id=a.tag_id
			WHERE a.site_id=$1 AND a.role='TERTIARY'`, siteID).Scan(&name); err != nil || name != "计算机" {
			t.Fatalf("imported current tag = %q: %v", name, err)
		}
	})
}
