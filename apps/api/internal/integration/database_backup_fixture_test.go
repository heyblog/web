//go:build integration

package integration_test

import (
	"strings"
	"testing"
	"time"

	"heyblog-api/internal/features/apikey"
	dbgen "heyblog-api/internal/infrastructure/database/gen"

	"github.com/jackc/pgx/v5/pgtype"
)

func seedDatabaseBackup(t *testing.T, fixture auditMigrationFixture, actor string, siteID pgtype.UUID) apikey.Credential {
	t.Helper()
	ctx := t.Context()
	queries := dbgen.New(fixture.pool)
	if err := queries.UpsertFriendLink(ctx, dbgen.UpsertFriendLinkParams{PSourceSiteID: siteID, PTargetHost: "external.example.test", PTargetUrl: "https://external.example.test/", PStatus: "INACTIVE"}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.admin.Exec(ctx, `LOAD 'age'; SET search_path=ag_catalog,public; SELECT * FROM ag_catalog.cypher('directory_graph',$$ CREATE (:SiteRef {normalized_host:'isolated.example.test'}) $$) AS (v agtype)`); err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`INSERT INTO identity.users(email,username,display_name,role,profile) VALUES('other@example.test','other_admin','Other','ADMIN',jsonb_build_object('actor_id',$1::text))`,
		`INSERT INTO identity.oauth_identities(user_id,provider,provider_user_id,provider_login) SELECT id,'GITHUB','synthetic-subject','synthetic-login' FROM identity.users WHERE role='ADMIN'`,
		`INSERT INTO identity.user_management_permissions(user_id,permission_key,granted_by) SELECT id,'site.manage',$1 FROM identity.users`,
		`INSERT INTO identity.email_verification_codes(user_id,email,code_hash,expires_at) SELECT id,email,'synthetic-code-hash',now()+interval '1 hour' FROM identity.users`,
		`INSERT INTO identity.password_reset_tokens(user_id,email,token_hash,expires_at) SELECT id,email,id::text,now()+interval '1 hour' FROM identity.users`,
		`INSERT INTO directory.site_icons(site_id,content,media_type,sha256) VALUES($2,decode('010203ff','hex'),'image/png',decode(repeat('ab',32),'hex'))`,
		`INSERT INTO directory.site_feeds(site_id,name,location_type,url_ref,url_key,is_default) VALUES($2,'Main','RELATIVE','/feed.xml','/feed.xml',true)`,
		`INSERT INTO directory.site_resources(site_id,kind,location_type,url_ref,url_key) VALUES($2,'SITEMAP','RELATIVE','/sitemap.xml','/sitemap.xml')`,
		`INSERT INTO directory.tag_slug_aliases(slug,tag_id) SELECT 'backup-historical-tag',id FROM directory.tags ORDER BY id LIMIT 1`,
		`INSERT INTO directory.site_tags(site_id,tag_id,role,note) SELECT $2,id,'WARNING','Historical warning' FROM directory.tags ORDER BY id LIMIT 1`,
		`INSERT INTO directory.tag_assignment_archive(scope,object_id,snapshot,reason) VALUES('SITE',$2,jsonb_build_object('actor_id',$1::text,'note','Historical assignment'),'synthetic-archive')`,
		`INSERT INTO directory.software_components(name,normalized_name) VALUES('Backup program','backup-program'),('Backup runtime','backup-runtime')`,
		`INSERT INTO directory.software_component_dependencies(component_id,dependency_component_id,role) SELECT p.id,r.id,'RUNTIME' FROM directory.software_components p CROSS JOIN directory.software_components r WHERE p.normalized_name='backup-program' AND r.normalized_name='backup-runtime'`,
		`INSERT INTO directory.site_software_components(site_id,component_id,role,identified_by) SELECT $2,id,'SITE_PROGRAM',$1 FROM directory.software_components WHERE normalized_name='backup-program'`,
		`INSERT INTO content.articles(site_id,location_type,url_ref,url_key,title,tag_cascade_id) SELECT $2,'RELATIVE','/historical-article','/historical-article','Historical article',id FROM directory.tag_cascades WHERE scope='ARTICLE' AND is_enabled ORDER BY id LIMIT 1`,
		`INSERT INTO content.article_tags(article_id,tag_id,role,note) SELECT a.id,t.id,'WARNING','Historical article warning' FROM content.articles a CROSS JOIN (SELECT id FROM directory.tags ORDER BY id LIMIT 1) t`,
		`INSERT INTO directory.site_metrics(site_id,click_count,query_count) VALUES($2,9007199254740991,1234)`,
		`INSERT INTO directory.site_metric_events(event_id,site_id,kind) VALUES(uuidv7(),$2,'CLICK')`,
		`INSERT INTO directory.slug_generation_jobs(owner_id,identity_ip_hash,model_id,status,revision,items) VALUES($1,repeat('a',64),'synthetic-model','paused',9007199254740993,'[{"status":"pending"}]')`,
		`INSERT INTO directory.slug_generation_cache(cache_key,slug) VALUES(repeat('b',64),'synthetic-slug')`,
		`INSERT INTO content.system_ai_settings(model_id) VALUES('synthetic-model')`,
		`INSERT INTO content.announcements(kind,title,status,starts_at,published_at,archived_at,created_by,updated_by,published_by,archived_by,row_version) VALUES('MAIN','Historical','ARCHIVED',now()-interval '2 days',now()-interval '2 days',now()-interval '1 day',$1,$1,$1,$1,9007199254740993)`,
		`INSERT INTO content.announcement_revisions(announcement_id,revision,kind,title,priority,action_type,starts_at,published_at,published_by,changed_by) SELECT id,9007199254740992,kind,'Original',0,'NONE',starts_at,published_at,$1,$1 FROM content.announcements`,
		`INSERT INTO directory.site_audits(lookup_secret_hash,action,status,site_id,base_revision,base_snapshot,proposed_snapshot,final_snapshot,request_reason,reviewed_by,reviewed_at) VALUES(decode(repeat('cd',32),'hex'),'UPDATE','APPROVED',$2,1,jsonb_build_object('confirmed_by',$1::text,'origins',jsonb_build_array(jsonb_build_object('metadata',jsonb_build_object('owner_id',$1::text)))),jsonb_build_object('confirmed_by',$1::text,'origins',jsonb_build_array(jsonb_build_object('metadata',jsonb_build_object('owner_id',$1::text)))),jsonb_build_object('confirmed_by',$1::text,'origins',jsonb_build_array(jsonb_build_object('metadata',jsonb_build_object('owner_id',$1::text)))),'Historical',$1,now())`,
		`UPDATE directory.sites SET visibility='HIDDEN',visibility_reason='Historical',custom_id='backup-blog' WHERE id=$2`,
		`UPDATE directory.tag_cascades SET is_enabled=false WHERE id=(SELECT tag_cascade_id FROM directory.sites WHERE id=$2)`,
	}
	for _, statement := range statements {
		// Session settings carry synthetic fixture identifiers into statements
		// without interpolating SQL literals.
		if _, err := fixture.admin.Exec(ctx, `SELECT set_config('backup.actor',$1,false),set_config('backup.site',$2,false)`, actor, integrationUUIDText(t, siteID)); err != nil {
			t.Fatal(err)
		}
		statement = strings.NewReplacer("$1", "current_setting('backup.actor')::uuid", "$2", "current_setting('backup.site')::uuid").Replace(statement)
		if strings.HasPrefix(statement, "UPDATE directory.sites SET visibility") {
			seedDatabaseBackupOwnership(t, fixture, actor, siteID)
		}
		if _, err := fixture.admin.Exec(ctx, statement); err != nil {
			t.Fatalf("seed backup: %v", err)
		}
	}
	keys := apikey.NewService(apikey.NewRepository(fixture.pool), time.Now)
	expires := time.Now().Add(24 * time.Hour)
	credential, err := keys.CreateClient(ctx, apikey.CreateClientRequest{Name: "Backup client", Audience: apikey.AudienceInternal, Scopes: []apikey.Scope{apikey.ScopeExampleCall}, ExpiresAt: &expires, CreatedBy: actor})
	if err != nil {
		t.Fatal(err)
	}
	return credential
}
