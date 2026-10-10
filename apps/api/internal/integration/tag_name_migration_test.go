//go:build integration

package integration_test

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestTagNameMigrationPreservesSelectedNamesAndHistory(t *testing.T) {
	// Given a changed default whose old label UUID is also a TAG identity.
	fixture := newAuditMigrationFixture(t)
	ctx := t.Context()
	if _, err := fixture.provider.UpTo(ctx, 27); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.admin.Exec(ctx, `LOAD 'age'`); err != nil {
		t.Fatal(err)
	}
	_, err := fixture.admin.Exec(ctx, `
BEGIN;
INSERT INTO directory.tags(id,slug,description,default_label_id,created_at,updated_at) VALUES
 ('10000000-0000-0000-0000-000000000001','first','shared description','10000000-0000-0000-0000-000000000001','2020-01-01','2020-02-01'),
 ('10000000-0000-0000-0000-000000000003','warning','warning description','10000000-0000-0000-0000-000000000003','2020-01-01','2020-02-01');
INSERT INTO directory.tag_labels(id,tag_id,name,is_enabled,created_at,updated_at) VALUES
 ('10000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000001','First name',true,'2021-01-01','2021-02-01'),
 ('10000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000001','Second name',true,'2022-01-01','2022-02-01'),
 ('10000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000003','Warning default',true,'2021-01-01','2021-02-01'),
 ('10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000003','Warning selected',false,'2022-01-01','2022-02-01'),
 ('10000000-0000-0000-0000-000000000005','10000000-0000-0000-0000-000000000003','Warning enabled label',true,'2022-01-01','2022-02-01');
UPDATE directory.tags SET is_enabled=false WHERE id='10000000-0000-0000-0000-000000000003';
UPDATE directory.tags SET default_label_id='10000000-0000-0000-0000-000000000002' WHERE id='10000000-0000-0000-0000-000000000001';
INSERT INTO directory.tag_identity_aliases(alias_id,tag_id,snapshot) VALUES
 ('10000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000001','{"name":"old TAG"}');
INSERT INTO directory.tag_cascades(id,scope,taxonomy_key,level1_tag_id,level2_tag_id,sort_order) VALUES
 ('20000000-0000-0000-0000-000000000001','SITE','managed/site-test','10000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000001',32767),
 ('20000000-0000-0000-0000-000000000002','ARTICLE','managed/article-test','10000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000001',32767);
INSERT INTO directory.sites(id,short_id,name,normalized_host,tag_cascade_id,primary_label_id,secondary_label_id)
 VALUES('30000000-0000-0000-0000-000000000001','NameOnly1','Selected site','selected.example.test','20000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000002');
INSERT INTO content.articles(id,site_id,location_type,url_ref,url_key,title,tag_cascade_id,primary_label_id,secondary_label_id)
 VALUES('30000000-0000-0000-0000-000000000002','30000000-0000-0000-0000-000000000001','RELATIVE','/post','/post','Selected article','20000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000001');
INSERT INTO directory.site_tags(site_id,tag_id,label_id,role,assignment_source,position,note,created_at)
 VALUES('30000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000004','TERTIARY','SYSTEM',1,'keep site note','2020-01-01');
INSERT INTO content.article_tags(article_id,tag_id,label_id,role,assignment_source,position,note,created_at)
 VALUES('30000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000004','TERTIARY','SYSTEM',1,'keep article note','2020-01-01');
UPDATE directory.tag_cascades SET is_enabled=false WHERE id IN('20000000-0000-0000-0000-000000000001','20000000-0000-0000-0000-000000000002');
INSERT INTO directory.tag_cascades(scope,taxonomy_key,level1_tag_id,level2_tag_id,sort_order,is_enabled,merged_into_id)
 VALUES('SITE','managed/merged-test','10000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000001',30001,false,'20000000-0000-0000-0000-000000000001');
INSERT INTO identity.users(id,email,username,display_name) VALUES('40000000-0000-0000-0000-000000000001','migration@example.test','migration_reviewer','Reviewer');
INSERT INTO directory.site_audits(lookup_secret_hash,action,status,site_id,base_revision,proposed_snapshot,request_reason,base_snapshot,review_draft_snapshot,final_snapshot,review_draft_revision,review_draft_updated_at,reviewed_by,reviewed_at)
 VALUES(decode(repeat('ab',32),'hex'),'UPDATE','APPROVED','30000000-0000-0000-0000-000000000001',1,'{"label_id":"10000000-0000-0000-0000-000000000001","slug":"first"}','keep history','{"name":"base"}','{"name":"draft"}','{"name":"final"}',1,now(),'40000000-0000-0000-0000-000000000001',now());
INSERT INTO directory.tag_assignment_archive(scope,object_id,snapshot,reason)
 VALUES('SITE','30000000-0000-0000-0000-000000000001','{"tag_id":"10000000-0000-0000-0000-000000000003"}','unrelated archive');
COMMIT;
CREATE TEMP TABLE before_tags AS SELECT * FROM directory.tags;
CREATE TEMP TABLE before_labels AS SELECT * FROM directory.tag_labels;
CREATE TEMP TABLE before_aliases AS SELECT * FROM directory.tag_identity_aliases;
CREATE TEMP TABLE before_paths AS SELECT * FROM directory.tag_cascades;
CREATE TEMP TABLE before_sites AS SELECT to_jsonb(s)-ARRAY['tag_cascade_id','primary_label_id','secondary_label_id'] AS row FROM directory.sites s;
CREATE TEMP TABLE before_articles AS SELECT to_jsonb(a)-ARRAY['tag_cascade_id','primary_label_id','secondary_label_id'] AS row FROM content.articles a;
CREATE TEMP TABLE before_site_tags AS SELECT to_jsonb(a)-ARRAY['tag_id','label_id'] AS row FROM directory.site_tags a;
CREATE TEMP TABLE before_article_tags AS SELECT to_jsonb(a)-ARRAY['tag_id','label_id'] AS row FROM content.article_tags a;
CREATE TEMP TABLE before_audits AS SELECT to_jsonb(a) AS row FROM directory.site_audits a;
CREATE TEMP TABLE before_archive AS SELECT to_jsonb(a) AS row FROM directory.tag_assignment_archive a;
`)
	if err != nil {
		t.Fatal(err)
	}
	// When the forward-only migration replaces labels with independent tags.
	if _, err := fixture.provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	// Then identity namespaces and every selected name remain unambiguous.
	checks := map[string]string{
		"disabled concept disables every name":     `SELECT count(*)=3 AND bool_and(NOT is_enabled) FROM directory.tags WHERE name IN('Warning default','Warning selected','Warning enabled label')`,
		"enabled concept and label remain enabled": `SELECT is_enabled FROM directory.tags WHERE name='First name'`,
		"original TAG aliases unchanged":           `SELECT NOT EXISTS(SELECT 1 FROM before_aliases b LEFT JOIN directory.tag_identity_aliases a ON a.alias_kind='TAG' AND a.alias_id=b.alias_id WHERE to_jsonb(b) IS DISTINCT FROM (to_jsonb(a)-'alias_kind'))`,
		"expanded integer ordering":                `SELECT count(*)=9 AND bool_and(c.sort_order>32767 AND c.taxonomy_key='managed/'||c.id::text) FROM directory.tag_cascades c WHERE NOT EXISTS(SELECT 1 FROM before_paths b WHERE b.id=c.id)`,
		"all names retained":                       `SELECT (SELECT count(*) FROM directory.tags)=(SELECT count(*) FROM before_labels)`,
		"default metadata retained":                `SELECT NOT EXISTS(SELECT 1 FROM before_tags b JOIN directory.tags t ON t.id=b.id WHERE (to_jsonb(b)-ARRAY['slug','default_label_id']) IS DISTINCT FROM (to_jsonb(t)-ARRAY['name','normalized_name']))`,
		"TAG and LABEL collision":                  `SELECT count(*)=2 AND count(DISTINCT tag_id)=2 FROM directory.tag_identity_aliases WHERE alias_id='10000000-0000-0000-0000-000000000001'`,
		"default UUID":                             `SELECT name='Second name' FROM directory.tags WHERE id='10000000-0000-0000-0000-000000000001'`,
		"selected SITE names":                      `SELECT p.name='First name' AND q.name='Second name' AND NOT c.is_enabled FROM directory.sites s JOIN directory.tag_cascades c ON c.id=s.tag_cascade_id JOIN directory.tags p ON p.id=c.level1_tag_id JOIN directory.tags q ON q.id=c.level2_tag_id WHERE s.short_id='NameOnly1'`,
		"selected ARTICLE names":                   `SELECT p.name='Second name' AND q.name='First name' AND NOT c.is_enabled FROM content.articles a JOIN directory.tag_cascades c ON c.id=a.tag_cascade_id JOIN directory.tags p ON p.id=c.level1_tag_id JOIN directory.tags q ON q.id=c.level2_tag_id WHERE a.title='Selected article'`,
		"ordered Cartesian roles":                  `SELECT count(*)=8 AND count(*) FILTER(WHERE c.is_enabled)=0 FROM directory.tag_cascades c JOIN directory.tags p ON p.id=c.level1_tag_id JOIN directory.tags q ON q.id=c.level2_tag_id WHERE p.name IN('First name','Second name') AND q.name IN('First name','Second name') AND c.merged_into_id IS NULL`,
		"merged history stays merged":              `SELECT count(*)=4 AND bool_and(NOT c.is_enabled) FROM directory.tag_cascades c JOIN directory.tags p ON p.id=c.level1_tag_id JOIN directory.tags q ON q.id=c.level2_tag_id WHERE p.name IN('First name','Second name') AND q.name IN('First name','Second name') AND c.merged_into_id IS NOT NULL`,
		"original paths unchanged":                 `SELECT NOT EXISTS(SELECT 1 FROM before_paths b JOIN directory.tag_cascades c ON c.id=b.id WHERE to_jsonb(b) IS DISTINCT FROM to_jsonb(c))`,
		"label snapshots exact":                    `SELECT count(*)=(SELECT count(*) FROM before_labels) AND bool_and(a.snapshot=to_jsonb(l)) FROM before_labels l JOIN directory.tag_identity_aliases a ON a.alias_kind='LABEL' AND a.alias_id=l.id`,
		"nondefault metadata":                      `SELECT t.description='shared description' AND t.created_at='2021-01-01'::timestamptz AND t.updated_at='2021-02-01'::timestamptz AND substring(t.id::text,15,1)='7' FROM directory.tags t WHERE name='First name'`,
		"disabled selected association":            `SELECT count(*)=1 AND bool_and(t.name='Warning selected' AND NOT t.is_enabled) FROM directory.site_tags a JOIN directory.tags t ON t.id=a.tag_id`,
		"article selected association":             `SELECT count(*)=1 AND bool_and(t.name='Warning selected' AND NOT t.is_enabled) FROM content.article_tags a JOIN directory.tags t ON t.id=a.tag_id`,
	}
	for name, query := range checks {
		t.Run(name, func(t *testing.T) {
			var ok bool
			if err := fixture.admin.QueryRow(ctx, query).Scan(&ok); err != nil || !ok {
				t.Fatalf("invariant = %v, error = %v", ok, err)
			}
		})
	}
	for _, pair := range []struct{ before, table, removed string }{
		{"before_sites", "directory.sites", "'tag_cascade_id'"},
		{"before_articles", "content.articles", "'tag_cascade_id'"},
		{"before_site_tags", "directory.site_tags", "'tag_id'"},
		{"before_article_tags", "content.article_tags", "'tag_id'"},
		{"before_audits", "directory.site_audits", "''"},
		{"before_archive", "directory.tag_assignment_archive", "''"},
	} {
		t.Run(pair.before, func(t *testing.T) {
			var ok bool
			query := "SELECT (SELECT jsonb_agg(row ORDER BY row::text) FROM " + pair.before + ")=(SELECT jsonb_agg(row ORDER BY row::text) FROM (SELECT to_jsonb(t)-" + pair.removed + " AS row FROM " + pair.table + " t) s)"
			if err := fixture.admin.QueryRow(ctx, query).Scan(&ok); err != nil || !ok {
				t.Fatalf("preserved rows = %v, error = %v", ok, err)
			}
		})
	}
	t.Run("case uniqueness", func(t *testing.T) {
		_, err := fixture.pool.Exec(ctx, `INSERT INTO directory.tags(name) VALUES('  FIRST NAME  ')`)
		var pgerr *pgconn.PgError
		if !errors.As(err, &pgerr) || pgerr.Code != "23505" {
			t.Fatalf("duplicate normalized name error = %v", err)
		}
	})
	t.Run("LABEL cannot carry system key", func(t *testing.T) {
		_, err := fixture.pool.Exec(ctx, `UPDATE directory.tag_identity_aliases SET system_key='invalid-label-key' WHERE alias_kind='LABEL' AND alias_id='10000000-0000-0000-0000-000000000001'`)
		var pgerr *pgconn.PgError
		if !errors.As(err, &pgerr) || pgerr.Code != "23514" {
			t.Fatalf("LABEL system key error = %v", err)
		}
	})
}
