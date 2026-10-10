//go:build integration

package integration_test

import (
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestTagNameDirectoryQueries(t *testing.T) {
	// Given a visible site with disabled existing tertiary and warning tags.
	f := newAuditMigrationFixture(t)
	ctx := t.Context()
	if _, err := f.provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	q := dbgen.New(f.pool)
	paths, err := q.ListEnabledSiteTagCascades(ctx)
	if err != nil || len(paths) == 0 {
		t.Fatalf("paths: %v", err)
	}
	c := paths[0]
	var siteID pgtype.UUID
	if err := f.pool.QueryRow(ctx, `INSERT INTO directory.sites(short_id,name,normalized_host,visibility,tag_cascade_id) VALUES('NameQuery','Query site','query.example.test','VISIBLE',$1) RETURNING id`, c.ID).Scan(&siteID); err != nil {
		t.Fatal(err)
	}
	tertiary, err := q.CreateTag(ctx, dbgen.CreateTagParams{Name: "Example", Description: "tertiary"})
	if err != nil {
		t.Fatal(err)
	}
	warning, err := q.CreateManagedTag(ctx, dbgen.CreateManagedTagParams{Name: "Caution", Description: "warning"})
	if err != nil {
		t.Fatal(err)
	}
	position := int16(1)
	if _, err = q.AssignSiteTag(ctx, dbgen.AssignSiteTagParams{SiteID: siteID, TagID: tertiary.ID, Role: "TERTIARY", AssignmentSource: "SYSTEM", Position: &position}); err != nil {
		t.Fatal(err)
	}
	if _, err = q.AssignSiteTag(ctx, dbgen.AssignSiteTagParams{SiteID: siteID, TagID: warning.ID, Role: "WARNING", AssignmentSource: "SYSTEM"}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(ctx, `UPDATE directory.tags SET is_enabled=false WHERE id=ANY($1::uuid[])`, []pgtype.UUID{tertiary.ID, warning.ID}); err != nil {
		t.Fatal(err)
	}
	// When generated directory queries execute against PostgreSQL.
	t.Run("count and list normalized filters", func(t *testing.T) {
		filter := dbgen.CountDirectorySitesByStatusParams{Level1TagName: " " + strings.ToUpper(c.Level1Name) + " ", Level2TagName: c.Level2Name, TertiaryTagNames: []string{"EXAMPLE", " example ", "Example"}, WarningNames: []string{" CAUTION ", "caution"}, TechnologyNames: []string{}, AccessScopes: []string{}, FeedMode: "any"}
		count, err := q.CountDirectorySitesByStatus(ctx, filter)
		if err != nil || count.NormalCount != 1 {
			t.Fatalf("count = %+v, %v", count, err)
		}
		rows, err := q.ListDirectorySites(ctx, dbgen.ListDirectorySitesParams{SiteVisibility: "VISIBLE", Level1TagName: filter.Level1TagName, Level2TagName: filter.Level2TagName, TertiaryTagNames: filter.TertiaryTagNames, WarningNames: filter.WarningNames, TechnologyNames: []string{}, AccessScopes: []string{}, FeedMode: "any", PageLimit: 10})
		if err != nil || len(rows) != 1 || rows[0].ID != siteID {
			t.Fatalf("list = %+v, %v", rows, err)
		}
		filter.TertiaryTagNames = []string{"exam"}
		count, err = q.CountDirectorySitesByStatus(ctx, filter)
		if err != nil || count.NormalCount != 0 {
			t.Fatalf("partial-name count = %+v, %v", count, err)
		}
	})
	// Then disabled names are still readable on existing associations.
	t.Run("read disabled assignments", func(t *testing.T) {
		rows, err := q.ListSiteTags(ctx, siteID)
		if err != nil || len(rows) != 2 {
			t.Fatalf("tags = %+v, %v", rows, err)
		}
		public, err := q.ListPublicSiteTags(ctx, siteID)
		if err != nil || len(public) != 4 {
			t.Fatalf("public tags = %+v, %v", public, err)
		}
		batch, err := q.ListPublicSiteTagsBySiteIDs(ctx, []pgtype.UUID{siteID})
		if err != nil || len(batch) != 4 {
			t.Fatalf("batch tags = %+v, %v", batch, err)
		}
		options, err := q.ListDirectoryTagOptions(ctx)
		if err != nil || len(options) != 4 {
			t.Fatalf("options = %+v, %v", options, err)
		}
		for _, o := range options {
			if o.Value != o.Name {
				t.Fatal("option value differs from name")
			}
		}
	})
	t.Run("name lookup and classification reads", func(t *testing.T) {
		tag, err := q.GetTagByNormalizedName(ctx, " EXAMPLE ")
		if err != nil || tag.ID != tertiary.ID {
			t.Fatalf("normalized lookup: %v", err)
		}
		if _, err := q.ListEnabledTags(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := q.ListManagedTags(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := q.GetCanonicalTag(ctx, tertiary.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := q.ListPublicSiteTagCascades(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := q.GetReadableSiteTagCascade(ctx, c.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := q.ReadableSiteCascades(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := q.PickRandomVisibleSite(ctx, dbgen.PickRandomVisibleSiteParams{Level1TagName: " " + c.Level1Name + " ", Level2TagName: c.Level2Name}); err != nil {
			t.Fatal(err)
		}
	})
}
