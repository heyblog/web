//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestDictionaryUpgradeFromDeployedAndDevelopment(t *testing.T) {
	for _, version := range []int64{16, 18} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			f := newAuditMigrationFixture(t)
			ctx := t.Context()
			if _, err := f.provider.UpTo(ctx, version); err != nil {
				t.Fatal(err)
			}
			var life, duplicate, site string
			if err := f.pool.QueryRow(ctx, `SELECT id::text FROM directory.tags WHERE system_key='life'`).Scan(&life); err != nil {
				t.Fatal(err)
			}
			if err := f.pool.QueryRow(ctx, `INSERT INTO directory.tags(name,normalized_name,slug) VALUES('生活','生活','legacy-life') RETURNING id::text`).Scan(&duplicate); err != nil {
				t.Fatal(err)
			}
			if err := f.pool.QueryRow(ctx, `INSERT INTO directory.sites(short_id,name,scheme,normalized_host,base_path,summary) VALUES('DiCt12345','Dictionary','https','dictionary.example.test','/','') RETURNING id::text`).Scan(&site); err != nil {
				t.Fatal(err)
			}
			if _, err := f.pool.Exec(ctx, `INSERT INTO directory.site_tags(site_id,tag_id,role,position) VALUES($1::uuid,$2::uuid,'TERTIARY',1),($1::uuid,$3::uuid,'TERTIARY',2)`, site, life, duplicate); err != nil {
				t.Fatal(err)
			}
			var article string
			if err := f.pool.QueryRow(ctx, `INSERT INTO content.articles(site_id,location_type,url_ref,url_key,tag_cascade_id) SELECT $1::uuid,'RELATIVE','/entry','/entry',id FROM directory.tag_cascades WHERE scope='ARTICLE' AND taxonomy_key='other/topic-other-other' RETURNING id::text`, site).Scan(&article); err != nil {
				t.Fatal(err)
			}
			if _, err := f.pool.Exec(ctx, `INSERT INTO content.article_tags(article_id,tag_id,role,position) VALUES($1::uuid,$2::uuid,'TERTIARY',1),($1::uuid,$3::uuid,'TERTIARY',2)`, article, life, duplicate); err != nil {
				t.Fatal(err)
			}
			pending := f.pendingCreate(t, "dictionary-pending.example.test")
			f.addReviewHistory(t, pending.AuditID)
			before := f.evidenceOutsideTaxonomy(t)
			if _, err := f.provider.UpTo(ctx, 19); err != nil {
				t.Fatal(err)
			}
			if after := f.evidenceOutsideTaxonomy(t); after != before {
				t.Fatal("audit history changed")
			}
			var canonical, slug string
			if err := f.pool.QueryRow(ctx, `SELECT tag_id::text FROM directory.tag_identity_aliases WHERE alias_id=$1::uuid`, duplicate).Scan(&canonical); err != nil || canonical != life {
				t.Fatalf("identity=%s error=%v", canonical, err)
			}
			if err := f.pool.QueryRow(ctx, `SELECT directory.canonical_tag_slug('legacy-life')`).Scan(&slug); err != nil || slug != "life" {
				t.Fatalf("slug=%s error=%v", slug, err)
			}
			var count, position int
			if err := f.pool.QueryRow(ctx, `SELECT count(*),min(position) FROM directory.site_tags WHERE site_id=$1::uuid`, site).Scan(&count, &position); err != nil || count != 1 || position != 1 {
				t.Fatalf("assignments=%d position=%d error=%v", count, position, err)
			}
			if err := f.pool.QueryRow(ctx, `SELECT count(*),min(position) FROM content.article_tags WHERE article_id=$1::uuid`, article).Scan(&count, &position); err != nil || count != 1 || position != 1 {
				t.Fatalf("article assignments=%d position=%d error=%v", count, position, err)
			}
			if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM directory.tag_assignment_archive WHERE object_id IN($1::uuid,$2::uuid)`, site, article).Scan(&count); err != nil || count != 4 {
				t.Fatal("lossless assignment archive", count, err)
			}
			var self bool
			if err := f.pool.QueryRow(ctx, `SELECT bool_and(level1_tag_id=level2_tag_id) FROM directory.tag_cascades WHERE taxonomy_key='other/topic-other-other'`).Scan(&self); err != nil || !self {
				t.Fatal("fallback not self-linked", err)
			}
			if _, err := f.pool.Exec(ctx, `INSERT INTO directory.tag_dictionary(name,normalized_name,slug,is_enabled) VALUES('生活','生活','another-life',false)`); err == nil {
				t.Fatal("disabled duplicate accepted")
			}
			if _, err := f.provider.DownTo(ctx, 18); err == nil || !strings.Contains(err.Error(), "restore the pre-upgrade") {
				t.Fatalf("unsafe rollback accepted: %v", err)
			}
		})
	}
}

func TestDictionaryMigrationConflictIsAtomic(t *testing.T) {
	f := newAuditMigrationFixture(t)
	ctx := t.Context()
	if _, err := f.provider.UpTo(ctx, 18); err != nil {
		t.Fatal(err)
	}
	var first, second, site string
	if err := f.pool.QueryRow(ctx, `SELECT id::text FROM directory.tags WHERE system_key='life'`).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(ctx, `INSERT INTO directory.tags(name,normalized_name,slug) VALUES('生活','生活','legacy-conflict') RETURNING id::text`).Scan(&second); err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(ctx, `INSERT INTO directory.sites(short_id,name,scheme,normalized_host,base_path,summary) VALUES('DiCf12345','Conflict','https','conflict.example.test','/','') RETURNING id::text`).Scan(&site); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO directory.site_tags(site_id,tag_id,role,position) VALUES($1::uuid,$2::uuid,'TERTIARY',1),($1::uuid,$3::uuid,'WARNING',NULL)`, site, first, second); err != nil {
		t.Fatal(err)
	}
	if _, err := f.provider.Up(ctx); err == nil || !strings.Contains(err.Error(), "conflicting assignment") {
		t.Fatalf("conflict not rejected: %v", err)
	}
	version, err := f.provider.GetDBVersion(ctx)
	if err != nil || version != 18 {
		t.Fatal("failed upgrade changed version", version, err)
	}
	var count int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM directory.site_tags WHERE site_id=$1::uuid`, site).Scan(&count); err != nil || count != 2 {
		t.Fatal("failed upgrade rewrote associations", count, err)
	}
	var absent bool
	if err := f.pool.QueryRow(ctx, `SELECT to_regclass('directory.tag_identity_aliases') IS NULL`).Scan(&absent); err != nil || !absent {
		t.Fatal("failed upgrade retained schema", err)
	}
}

func TestDictionaryBackupRestoreRoundTrip(t *testing.T) {
	f := newAuditMigrationFixture(t)
	ctx := t.Context()
	if _, err := f.provider.UpTo(ctx, 18); err != nil {
		t.Fatal(err)
	}
	var before int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM directory.tags`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	code, output, err := f.container.Exec(ctx, []string{"pg_dump", "-U", "postgres", "-d", "heyblog", "-Fc", "-f", "/tmp/pre-dictionary.dump"})
	if err != nil || code != 0 {
		body, _ := io.ReadAll(output)
		t.Fatalf("backup failed code=%d err=%v output=%s", code, err, body)
	}
	if _, err = f.provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = f.admin.Exec(ctx, `CREATE DATABASE dictionary_restore`); err != nil {
		t.Fatal(err)
	}
	code, output, err = f.container.Exec(ctx, []string{"pg_restore", "-U", "postgres", "-d", "dictionary_restore", "--exit-on-error", "/tmp/pre-dictionary.dump"})
	if err != nil || code != 0 {
		body, _ := io.ReadAll(output)
		t.Fatalf("restore failed code=%d err=%v output=%s", code, err, body)
	}
	cfg := f.admin.Config().Copy()
	cfg.Database = "dictionary_restore"
	restored, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = restored.Close(context.Background()) }()
	var after, version int
	if err = restored.QueryRow(ctx, `SELECT count(*) FROM directory.tags`).Scan(&after); err != nil || after != before {
		t.Fatal("restored dictionary differs", before, after, err)
	}
	if err = restored.QueryRow(ctx, `SELECT max(version_id) FROM migration.goose_db_version WHERE is_applied`).Scan(&version); err != nil || version != 18 {
		t.Fatal("restored version differs", version, err)
	}
	var self bool
	if err = restored.QueryRow(ctx, `SELECT bool_or(level1_tag_id=level2_tag_id) FROM directory.tag_cascades`).Scan(&self); err != nil || self {
		t.Fatal("restore retained consolidated relations", self, err)
	}
}
