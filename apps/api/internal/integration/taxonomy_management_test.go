//go:build integration

package integration_test

import (
	"testing"

	"heyblog-api/internal/features/dataimport"
	"heyblog-api/internal/features/taxonomy"
	dbgen "heyblog-api/internal/infrastructure/database/gen"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestTaxonomyManagement(t *testing.T) {
	f := newAuditMigrationFixture(t)
	ctx := t.Context()
	if _, err := f.provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	service := taxonomy.NewService(f.pool, nil)
	catalog, err := service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	create := func(name string, level int16, parent string) taxonomy.Tag {
		t.Helper()
		var err error
		catalog, err = service.Create(ctx, taxonomy.CreateInput{Name: name, ExpectedRevision: catalog.Revision})
		if err != nil {
			t.Fatalf("create %s: %+v", name, err)
		}
		if level == 2 {
			var id string
			for _, tag := range catalog.Tags {
				if tag.Name == name {
					id = tag.ID
				}
			}
			for _, scope := range []string{"SITE", "ARTICLE"} {
				catalog, err = service.CreateCascade(ctx, taxonomy.CreateCascadeInput{Scope: scope, PrimaryID: parent, SecondaryID: id, Key: "test/" + name, ExpectedRevision: catalog.Revision})
				if err != nil {
					t.Fatal(err)
				}
			}
		}

		for _, tag := range catalog.Tags {
			if tag.Name == name {
				return tag
			}
		}
		t.Fatal("created tag missing")
		return taxonomy.Tag{}
	}
	first := create("test-primary", 1, "")
	second := create("test-other-primary", 1, "")
	child := create("test-child", 2, first.ID)
	target := create("test-target", 2, second.ID)
	tertiary := create("test-old-tertiary", 3, "")
	canonical := create("test-new-tertiary", 3, "")
	path := func(tag, scope string) string {
		for _, c := range catalog.Cascades {
			if c.SecondaryID == tag && c.Scope == scope && c.MergedIntoID == "" {
				return c.ID
			}
		}
		t.Fatalf("missing %s path", scope)
		return ""
	}
	var siteID, articleID string
	if err := f.pool.QueryRow(ctx, `INSERT INTO directory.sites(short_id,name,scheme,normalized_host,base_path,summary,tag_cascade_id) VALUES('Taxo12345','Taxonomy','https','taxonomy.example.test','/','',$1::uuid) RETURNING id::text`, path(child.ID, "SITE")).Scan(&siteID); err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(ctx, `INSERT INTO content.articles(site_id,location_type,url_ref,url_key,tag_cascade_id) VALUES($1::uuid,'RELATIVE','/article','/article',$2::uuid) RETURNING id::text`, siteID, path(child.ID, "ARTICLE")).Scan(&articleID); err != nil {
		t.Fatal(err)
	}
	for _, assignment := range []struct{ table, column, id string }{{"directory.site_tags", "site_id", siteID}, {"content.article_tags", "article_id", articleID}} {
		if _, err := f.pool.Exec(ctx, `INSERT INTO `+assignment.table+` (`+assignment.column+`,tag_id,role,position) VALUES($1::uuid,$2::uuid,'TERTIARY',1),($1::uuid,$3::uuid,'TERTIARY',2)`, assignment.id, tertiary.ID, canonical.ID); err != nil {
			t.Fatal(err)
		}
	}
	apply := func(input taxonomy.ChangeInput) taxonomy.Preview {
		t.Helper()
		catalog, err = service.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		input.ExpectedRevision = catalog.Revision
		preview, err := service.Preview(ctx, input)
		if err != nil || len(preview.Blockers) > 0 {
			t.Fatalf("preview: %+v %v", preview, err)
		}
		input.Fingerprint = preview.Fingerprint
		catalog, err = service.Apply(ctx, input)
		if err != nil {
			t.Fatalf("apply %s: %+v", input.Kind, err)
		}
		return preview
	}
	preview := apply(taxonomy.ChangeInput{Kind: "merge", SourceID: tertiary.ID, TargetID: canonical.ID})
	if preview.SiteCount != 1 || preview.ArticleCount != 1 || preview.RemovedDuplicates != 2 {
		t.Fatalf("merge effects: %#v", preview)
	}
	var count int
	var position int
	if err := f.pool.QueryRow(ctx, `SELECT count(*),min(position) FROM directory.site_tags WHERE site_id=$1::uuid`, siteID).Scan(&count, &position); err != nil || count != 1 || position != 1 {
		t.Fatalf("dedup count=%d position=%d error=%v", count, position, err)
	}
	for _, scope := range []string{"SITE", "ARTICLE"} {
		apply(taxonomy.ChangeInput{Kind: "path_update", SourceID: path(child.ID, scope), PrimaryID: second.ID, SecondaryID: child.ID, Enabled: true})
	}
	for _, scope := range []string{"SITE", "ARTICLE"} {
		var parent string
		if err := f.pool.QueryRow(ctx, `SELECT level1_tag_id::text FROM directory.tag_cascades WHERE id=$1::uuid`, path(child.ID, scope)).Scan(&parent); err != nil || parent != second.ID {
			t.Fatalf("reparent %s = %s %v", scope, parent, err)
		}
	}
	oldPath := path(child.ID, "SITE")
	apply(taxonomy.ChangeInput{Kind: "merge", SourceID: child.ID, TargetID: target.ID})
	var cascadeID string
	if err := f.pool.QueryRow(ctx, `SELECT tag_cascade_id::text FROM directory.sites WHERE id=$1::uuid`, siteID).Scan(&cascadeID); err != nil || cascadeID != path(target.ID, "SITE") {
		t.Fatalf("merged path %s %v", cascadeID, err)
	}
	var alias string
	if err := f.pool.QueryRow(ctx, `SELECT merged_into_id::text FROM directory.tag_cascades WHERE id=$1::uuid`, oldPath).Scan(&alias); err != nil || alias != cascadeID {
		t.Fatalf("old path alias %s %v", alias, err)
	}
	oldRevision := catalog.Revision
	catalog, err = service.Update(ctx, canonical.ID, taxonomy.UpdateInput{Name: "Renamed", Enabled: true, ExpectedRevision: catalog.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Update(ctx, canonical.ID, taxonomy.UpdateInput{Name: "Stale", Enabled: true, ExpectedRevision: oldRevision}); err == nil {
		t.Fatal("stale mutation succeeded")
	}
	var importedPathKey string
	for _, c := range catalog.Cascades {
		if c.ID == oldPath {
			importedPathKey = c.Key
		}
	}
	_, err = dataimport.NewRepository(f.pool).ImportTaxonomy(ctx, dataimport.TagTaxonomyBundle{
		Tags:  []dataimport.TagTaxonomyDefinition{{Source: "LEGACY", TagID: tertiary.ID, Name: "Must not overwrite"}},
		Sites: []dataimport.SiteTagTaxonomyMigration{{SiteID: siteID, CascadeKey: importedPathKey, TertiaryTags: []dataimport.TagTaxonomyTertiary{{Source: "LEGACY", TagID: tertiary.ID, Position: 1}, {Source: "LEGACY", TagID: canonical.ID, Position: 2}}}},
	})
	if err != nil {
		t.Fatalf("import historical aliases: %v", err)
	}
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM directory.site_tags WHERE site_id=$1::uuid AND tag_id=$2::uuid`, siteID, canonical.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("import dedup count=%d %v", count, err)
	}
	catalog, err = service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err = service.Update(ctx, target.ID, taxonomy.UpdateInput{Name: target.Name, Enabled: false, ExpectedRevision: catalog.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(ctx, `UPDATE directory.sites SET name='Still readable',tag_cascade_id=tag_cascade_id WHERE id=$1::uuid`, siteID); err != nil {
		t.Fatalf("unchanged disabled assignment rejected: %v", err)
	}
	catalog, err = service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	warning := create("test-disabled-warning", 3, "")
	if _, err = f.pool.Exec(ctx, `INSERT INTO directory.site_tags(site_id,tag_id,role) VALUES($1::uuid,$2::uuid,'WARNING')`, siteID, warning.ID); err != nil {
		t.Fatal(err)
	}
	catalog, err = service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, tag := range []taxonomy.Tag{second, {ID: canonical.ID, Name: "Renamed"}, warning} {
		catalog, err = service.Update(ctx, tag.ID, taxonomy.UpdateInput{Name: tag.Name, Enabled: false, ExpectedRevision: catalog.Revision})
		if err != nil {
			t.Fatal(err)
		}
	}
	queries := dbgen.New(f.pool)
	var siteUUID pgtype.UUID
	if err = siteUUID.Scan(siteID); err != nil {
		t.Fatal(err)
	}
	publicTags, err := queries.ListPublicSiteTags(ctx, siteUUID)
	if err != nil || len(publicTags) != 4 {
		t.Fatalf("disabled public tags: %#v %v", publicTags, err)
	}
	batched, err := queries.ListPublicSiteTagsBySiteIDs(ctx, []pgtype.UUID{siteUUID})
	if err != nil || len(batched) != 4 {
		t.Fatalf("disabled batched tags: %#v %v", batched, err)
	}
	options, err := queries.ListDirectoryTagOptions(ctx)
	if err != nil || len(options) != 4 {
		t.Fatalf("disabled filter options: %#v %v", options, err)
	}
	counts, err := queries.CountDirectorySitesByStatus(ctx, dbgen.CountDirectorySitesByStatusParams{Level1TagName: second.Name, Level2TagName: target.Name, TertiaryTagNames: []string{"Renamed", "RENAMED"}, WarningNames: []string{warning.Name}, TechnologyNames: []string{}, AccessScopes: []string{}, FeedMode: "any"})
	if err != nil || counts.NormalCount != 1 {
		t.Fatalf("disabled name count: %#v %v", counts, err)
	}
	sites, err := queries.ListDirectorySites(ctx, dbgen.ListDirectorySitesParams{SiteVisibility: "VISIBLE", Level1TagName: second.Name, Level2TagName: target.Name, TertiaryTagNames: []string{"Renamed", "RENAMED"}, WarningNames: []string{warning.Name}, TechnologyNames: []string{}, AccessScopes: []string{}, FeedMode: "any", SortMode: "name", SortOrder: "asc", PageLimit: 20})
	if err != nil || len(sites) != 1 || sites[0].ID != siteUUID {
		t.Fatalf("disabled name list: %#v %v", sites, err)
	}
	random, err := queries.PickRandomVisibleSite(ctx, dbgen.PickRandomVisibleSiteParams{Level1TagName: second.Name, Level2TagName: target.Name})
	if err != nil || random.ID != siteUUID {
		t.Fatalf("disabled random selection: %#v %v", random, err)
	}
	readable, err := queries.ListPublicSiteTagCascades(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range readable {
		found = found || c.Level2Name == target.Name
	}
	if !found {
		t.Fatal("disabled classification missing from public options")
	}
	selectable, err := queries.ListEnabledSiteTagCascades(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range selectable {
		if c.Level2Name == target.Name {
			t.Fatal("disabled classification allowed as a new selection")
		}
	}
	enabledTags, err := queries.ListEnabledTags(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, tag := range enabledTags {
		if tag.Name == "Renamed" || tag.Name == warning.Name {
			t.Fatal("disabled tag allowed as a new selection")
		}
	}
	verifyTaxonomyAuditReferences(ctx, t, f, service)
}
