//go:build integration

package integration_test

import (
	"testing"
)

func TestHistoricalTagLabelsUpgradePreservesData(t *testing.T) {
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
	if _, err := f.provider.UpTo(ctx, 22); err != nil {
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
	if err != nil || version != 22 {
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
}
