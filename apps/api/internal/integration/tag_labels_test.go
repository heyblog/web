//go:build integration

package integration_test

import (
	"testing"

	"heyblog-api/internal/features/dataimport"
	"heyblog-api/internal/features/publicview"
	"heyblog-api/internal/features/taxonomy"
	dbgen "heyblog-api/internal/infrastructure/database/gen"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestTagLabelsUpgradeAndSemanticMerge(t *testing.T) {
	f := newAuditMigrationFixture(t)
	ctx := t.Context()
	if _, err := f.provider.UpTo(ctx, 21); err != nil {
		t.Fatal(err)
	}
	var source, target, siteA, siteB, siteC string
	for _, entry := range []struct {
		name, slug string
		id         *string
	}{{"算法", "algorithm-old", &source}, {"algorithm", "algorithm", &target}} {
		if err := f.pool.QueryRow(ctx, `INSERT INTO directory.tags(name,normalized_name,slug) VALUES($1,lower($1),$2) RETURNING id::text`, entry.name, entry.slug).Scan(entry.id); err != nil {
			t.Fatal(err)
		}
	}
	for i, id := range []*string{&siteA, &siteB, &siteC} {
		if err := f.pool.QueryRow(ctx, `INSERT INTO directory.sites(short_id,name,scheme,normalized_host,base_path,summary) VALUES($1,'Semantic','https',$2,'/','') RETURNING id::text`, []string{"LaBl12345", "LaBl12346", "LaBl12347"}[i], []string{"label-a.example.test", "label-b.example.test", "label-c.example.test"}[i]).Scan(id); err != nil {
			t.Fatal(err)
		}
	}
	for _, a := range []struct {
		site, tag string
		position  int
	}{{siteA, source, 1}, {siteB, target, 1}, {siteC, source, 1}, {siteC, target, 2}} {
		if _, err := f.pool.Exec(ctx, `INSERT INTO directory.site_tags(site_id,tag_id,role,position) VALUES($1::uuid,$2::uuid,'TERTIARY',$3)`, a.site, a.tag, a.position); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.pool.Exec(ctx, `WITH inserted AS (INSERT INTO directory.site_metrics(site_id,click_count,impression_count,response_count,query_count) VALUES($1::uuid,7,11,13,17) RETURNING site_id) INSERT INTO directory.site_metric_events(event_id,site_id,kind) SELECT uuidv7(),site_id,'CLICK' FROM inserted`, siteA); err != nil {
		t.Fatal(err)
	}
	var pathA, pathB string
	for i, entry := range []struct {
		tag  string
		site *string
	}{{source, &pathA}, {target, &pathB}} {
		var cascade string
		if err := f.pool.QueryRow(ctx, `INSERT INTO directory.tag_cascades(scope,taxonomy_key,level1_tag_id,level2_tag_id,sort_order) SELECT 'SITE',$1,$2::uuid,id,$3::smallint FROM directory.tags WHERE slug='other' RETURNING id::text`, []string{"fixture/labels-source", "fixture/labels-target"}[i], entry.tag, 500+i).Scan(&cascade); err != nil {
			t.Fatal(err)
		}
		if err := f.pool.QueryRow(ctx, `INSERT INTO directory.sites(short_id,name,scheme,normalized_host,base_path,summary,tag_cascade_id) VALUES($1,'Semantic path','https',$2,'/','',$3::uuid) RETURNING id::text`, []string{"LaBl12348", "LaBl12349"}[i], []string{"label-path-a.example.test", "label-path-b.example.test"}[i], cascade).Scan(entry.site); err != nil {
			t.Fatal(err)
		}
	}
	pending := f.pendingCreate(t, "labels-history.example.test")
	var historyBefore string
	if err := f.pool.QueryRow(ctx, `SELECT proposed_snapshot::text FROM directory.site_audits WHERE id=$1::uuid`, pending.AuditID).Scan(&historyBefore); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO directory.slug_generation_jobs(identity_ip_hash,model_id,items,status) VALUES(repeat('a',64),'fixture','[{"state":"ready"}]','ready')`); err != nil {
		t.Fatal(err)
	}
	var metadataBefore, metadataAfter string
	metadataQuery := `SELECT md5(jsonb_build_array((SELECT jsonb_agg(jsonb_build_array(id,revision,updated_at) ORDER BY id) FROM directory.sites),(SELECT jsonb_agg(jsonb_build_array(id,created_at,updated_at) ORDER BY id) FROM directory.tags),(SELECT jsonb_agg(jsonb_build_array(id,updated_at) ORDER BY id) FROM content.articles))::text)`
	if err := f.pool.QueryRow(ctx, metadataQuery).Scan(&metadataBefore); err != nil {
		t.Fatal(err)
	}
	if _, err := f.provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(ctx, metadataQuery).Scan(&metadataAfter); err != nil || metadataAfter != metadataBefore {
		t.Fatal("label backfill changed edit timestamps or optimistic revisions", err)
	}
	var disabledTriggers int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM pg_trigger WHERE tgname IN ('sites_touch_site','tags_touch_updated_at','articles_touch_updated_at') AND tgenabled<>'O'`).Scan(&disabledTriggers); err != nil || disabledTriggers != 0 {
		t.Fatal("label migration left metadata triggers disabled", err)
	}
	version, err := f.provider.GetDBVersion(ctx)
	if err != nil || version != 25 {
		t.Fatalf("tag labels migration version=%d err=%v", version, err)
	}
	var clicks, impressions, responses, queries, events int64
	if err := f.pool.QueryRow(ctx, `SELECT click_count,impression_count,response_count,query_count,(SELECT count(*) FROM directory.site_metric_events WHERE site_id=$1::uuid) FROM directory.site_metrics WHERE site_id=$1::uuid`, siteA).Scan(&clicks, &impressions, &responses, &queries, &events); err != nil || clicks != 7 || impressions != 11 || responses != 13 || queries != 17 || events != 1 {
		t.Fatalf("label upgrade changed metrics: %d/%d/%d/%d events=%d err=%v", clicks, impressions, responses, queries, events, err)
	}
	var historyAfter, state string
	if err := f.pool.QueryRow(ctx, `SELECT proposed_snapshot::text FROM directory.site_audits WHERE id=$1::uuid`, pending.AuditID).Scan(&historyAfter); err != nil {
		t.Fatal(err)
	}
	if historyAfter != historyBefore {
		t.Fatal("migration rewrote immutable audit snapshot")
	}
	if err := f.pool.QueryRow(ctx, `SELECT status FROM directory.slug_generation_jobs`).Scan(&state); err != nil || state != "cancelled" {
		t.Fatalf("unfinished job=%s err=%v", state, err)
	}
	var previewJob string
	if err := f.pool.QueryRow(ctx, `INSERT INTO directory.slug_generation_jobs(identity_ip_hash,model_id,items,status) VALUES(repeat('b',64),'fixture',jsonb_build_array(jsonb_build_object('tag_id',$1::text,'state','needs_confirmation','conflicts',jsonb_build_array(jsonb_build_object('id',$2::text)))),'ready') RETURNING id::text`, source, target).Scan(&previewJob); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO directory.slug_generation_jobs(identity_ip_hash,model_id,items,status) VALUES(repeat('c',64),'fixture',jsonb_build_array(jsonb_build_object('tag_id',$1::text,'state','ready','conflicts',NULL)),'ready')`, target); err != nil {
		t.Fatal(err)
	}
	service := taxonomy.NewService(f.pool, nil)
	catalog, err := service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var sourceLabel, targetLabel string
	for _, tag := range catalog.Tags {
		if tag.ID == source {
			sourceLabel = tag.DefaultLabelID
		}
		if tag.ID == target {
			targetLabel = tag.DefaultLabelID
		}
	}
	input := taxonomy.ChangeInput{Kind: "merge", SourceID: source, TargetID: target, ExpectedRevision: catalog.Revision}
	preview, err := service.Preview(ctx, input)
	if err != nil || len(preview.Blockers) > 0 {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	input.Fingerprint = preview.Fingerprint
	catalog, err = service.Apply(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []struct{ site, label, name string }{{siteA, sourceLabel, "算法"}, {siteB, targetLabel, "algorithm"}, {siteC, targetLabel, "algorithm"}} {
		var gotTag, gotLabel, gotName string
		if err := f.pool.QueryRow(ctx, `SELECT a.tag_id::text,a.label_id::text,l.name FROM directory.site_tags a JOIN directory.tag_labels l ON l.id=a.label_id WHERE a.site_id=$1::uuid`, expected.site).Scan(&gotTag, &gotLabel, &gotName); err != nil {
			t.Fatal(err)
		}
		if gotTag != target || gotLabel != expected.label || gotName != expected.name {
			t.Fatalf("assignment=%s/%s/%s expected=%+v", gotTag, gotLabel, gotName, expected)
		}
	}
	for _, expected := range []struct{ site, label string }{{pathA, sourceLabel}, {pathB, targetLabel}} {
		var got string
		if err := f.pool.QueryRow(ctx, `SELECT primary_label_id::text FROM directory.sites WHERE id=$1::uuid`, expected.site).Scan(&got); err != nil || got != expected.label {
			t.Fatalf("primary selection=%s expected=%s err=%v", got, expected.label, err)
		}
	}
	if err := f.pool.QueryRow(ctx, `SELECT items->0->>'state' FROM directory.slug_generation_jobs WHERE id=$1::uuid`, previewJob).Scan(&state); err != nil || state != "stale" {
		t.Fatalf("merge retained outdated preview=%s err=%v", state, err)
	}
	q := dbgen.New(f.pool)
	if _, err := f.pool.Exec(ctx, `INSERT INTO directory.site_software_components(site_id,component_id,role) SELECT $1::uuid,id,'SITE_PROGRAM' FROM directory.software_components WHERE normalized_name='其他'`, siteA); err != nil {
		t.Fatal(err)
	}
	options, err := publicview.New(q, nil).DirectoryOptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	synonyms := map[string]bool{}
	for _, option := range options.TertiaryTags {
		if option.Value == "algorithm" {
			synonyms[option.Label] = true
			if option.NormalCount != 3 {
				t.Fatalf("synonym count=%+v", option)
			}
		}
	}
	if !synonyms["算法"] || !synonyms["algorithm"] || len(options.Technologies) == 0 {
		t.Fatalf("full options omitted synonym or technologies: %+v", options)
	}
	for _, term := range []string{"算法", "algorithm"} {
		counts, err := q.CountDirectorySitesByStatus(ctx, dbgen.CountDirectorySitesByStatusParams{QueryText: term, FeedMode: "any", TertiaryTagSlugs: []string{}, WarningSlugs: []string{}, TechnologyNames: []string{}, AccessScopes: []string{}})
		if err != nil || counts.NormalCount != 5 {
			t.Fatalf("synonym search %s = %+v,%v", term, counts, err)
		}
	}
	other, err := q.ResolveDirectoryLabel(ctx, dbgen.ResolveDirectoryLabelParams{Slug: "other", LabelIds: []string{sourceLabel, targetLabel}})
	if err != nil || integrationUUIDText(t, other.LabelID) == sourceLabel || integrationUUIDText(t, other.LabelID) == targetLabel {
		t.Fatalf("foreign name selected: %+v,%v", other, err)
	}
	selected, err := q.ResolveDirectoryLabel(ctx, dbgen.ResolveDirectoryLabelParams{Slug: "algorithm", LabelIds: []string{integrationUUIDText(t, other.LabelID), sourceLabel}})
	if err != nil || integrationUUIDText(t, selected.LabelID) != sourceLabel {
		t.Fatalf("explicit synonym lost after missing first name: %+v,%v", selected, err)
	}
	counts, err := q.CountDirectorySitesByStatus(ctx, dbgen.CountDirectorySitesByStatusParams{FeedMode: "any", TertiaryTagSlugs: []string{"algorithm-old", "algorithm"}, WarningSlugs: []string{}, TechnologyNames: []string{}, AccessScopes: []string{}})
	if err != nil || counts.NormalCount != 3 {
		t.Fatalf("synonym filters = %+v,%v", counts, err)
	}
	importer := dataimport.NewRepository(f.pool)
	for range 2 {
		_, err := importer.ImportTaxonomy(ctx, dataimport.TagTaxonomyBundle{Sites: []dataimport.SiteTagTaxonomyMigration{
			{SiteID: siteA, CascadeKey: "other/topic-other-other", TertiaryTags: []dataimport.TagTaxonomyTertiary{{Source: "LEGACY", TagID: source, Name: "算法", Position: 1}}},
			{SiteID: pathA, CascadeKey: "fixture/labels-source"},
		}})
		if err != nil {
			t.Fatal(err)
		}
		var tertiaryLabel, primaryLabel string
		if err := f.pool.QueryRow(ctx, `SELECT label_id::text FROM directory.site_tags WHERE site_id=$1::uuid AND tag_id=$2::uuid`, siteA, target).Scan(&tertiaryLabel); err != nil || tertiaryLabel != sourceLabel {
			t.Fatalf("reimport lost tertiary name: %s,%v", tertiaryLabel, err)
		}
		if err := f.pool.QueryRow(ctx, `SELECT primary_label_id::text FROM directory.sites WHERE id=$1::uuid`, pathA).Scan(&primaryLabel); err != nil || primaryLabel != sourceLabel {
			t.Fatalf("reimport lost primary name: %s,%v", primaryLabel, err)
		}
	}
	catalog, err = service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.DeleteLabel(ctx, target, sourceLabel, taxonomy.DeleteInput{ExpectedRevision: catalog.Revision}); err == nil {
		t.Fatal("referenced label deleted")
	}
	catalog, err = service.UpdateLabel(ctx, target, sourceLabel, taxonomy.LabelInput{Name: "算法", Enabled: false, ExpectedRevision: catalog.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.DefaultLabel(ctx, target, taxonomy.DefaultLabelInput{LabelID: sourceLabel, ExpectedRevision: catalog.Revision}); err == nil {
		t.Fatal("disabled default accepted")
	}
	var aUUID pgtype.UUID
	if err := aUUID.Scan(siteA); err != nil {
		t.Fatal(err)
	}
	tags, err := q.ListPublicSiteTags(ctx, aUUID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tag := range tags {
		if tag.Role == "TERTIARY" && tag.Name == "算法" {
			found = true
		}
	}
	if !found {
		t.Fatal("disabled selected name stopped displaying")
	}
	if _, err := f.pool.Exec(ctx, `UPDATE directory.site_tags SET label_id=(SELECT default_label_id FROM directory.tags WHERE slug='other') WHERE site_id=$1::uuid`, siteA); err == nil {
		t.Fatal("forged label ownership accepted")
	}
}
