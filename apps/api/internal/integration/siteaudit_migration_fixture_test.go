//go:build integration

package integration_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	dbgen "heyblog-api/internal/database/gen"
	"heyblog-api/internal/database/migrations"
	"heyblog-api/internal/siteaudit"
)

type auditMigrationFixture struct {
	provider *goose.Provider
	pool     *pgxpool.Pool
	admin    *pgx.Conn
}

func newAuditMigrationFixture(t *testing.T) auditMigrationFixture {
	t.Helper()
	ctx := t.Context()
	container, err := postgres.Run(ctx, "apache/age:release_PG18_1.7.0",
		postgres.WithDatabase("heyblog"), postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres-secret"), postgres.BasicWaitStrategies())
	if err != nil {
		t.Fatalf("start audit migration database: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Errorf("terminate audit migration database: %v", err)
		}
	})
	adminURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get audit migration connection: %v", err)
	}
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect audit migration administrator: %v", err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	bootstrapDatabaseRoles(ctx, t, admin)
	configuration, err := pgx.ParseConfig(databaseURLForRole(t, adminURL, "migrator", "migrator-secret"))
	if err != nil {
		t.Fatalf("parse audit migration connection: %v", err)
	}
	db := sql.OpenDB(stdlib.GetConnector(*configuration))
	t.Cleanup(func() { _ = db.Close() })
	migrationFS, err := migrations.Filesystem()
	if err != nil {
		t.Fatalf("open audit migrations: %v", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrationFS,
		goose.WithTableName("migration.goose_db_version"))
	if err != nil {
		t.Fatalf("create audit migration provider: %v", err)
	}
	pool, err := pgxpool.New(ctx, databaseURLForRole(t, adminURL, "api_runtime", "runtime-secret"))
	if err != nil {
		t.Fatalf("connect audit runtime: %v", err)
	}
	t.Cleanup(pool.Close)
	return auditMigrationFixture{provider: provider, pool: pool, admin: admin}
}

func (fixture auditMigrationFixture) pendingCreate(t *testing.T, host string) siteaudit.SubmissionResult {
	t.Helper()
	ctx := t.Context()
	program, err := dbgen.New(fixture.pool).GetSoftwareComponentByNormalizedName(ctx, "其他")
	if err != nil {
		t.Fatalf("get audit fixture program: %v", err)
	}
	snapshot := siteaudit.Snapshot{
		Name: "Historical submission", Scheme: "https", NormalizedHost: host,
		BasePath: "/", AccessScope: "ALL", Visibility: "VISIBLE",
		Tags:       []siteaudit.TagSnapshot{},
		Components: []siteaudit.ComponentSnapshot{{ID: integrationUUIDText(t, program.ID), Role: "SITE_PROGRAM"}},
	}
	version, err := fixture.provider.GetDBVersion(ctx)
	if err != nil {
		t.Fatalf("get fixture migration version: %v", err)
	}
	if version >= 11 {
		cascades, listErr := dbgen.New(fixture.pool).ListEnabledSiteTagCascades(ctx)
		if listErr != nil || len(cascades) == 0 {
			t.Fatalf("get audit fixture cascade: %v", listErr)
		}
		cascade := cascades[0]
		snapshot.TagCascadeID = integrationUUIDText(t, cascade.ID)
		snapshot.Tags = []siteaudit.TagSnapshot{
			{ID: integrationUUIDText(t, cascade.Level1ID), Role: "PRIMARY", Level: 1},
			{ID: integrationUUIDText(t, cascade.Level2ID), Role: "SECONDARY", Level: 2, ParentID: integrationUUIDText(t, cascade.Level1ID)},
		}
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("encode historical audit fixture: %v", err)
	}
	secret := sha256.Sum256([]byte("migration-fixture-" + host))
	result := siteaudit.SubmissionResult{LookupToken: base64.RawURLEncoding.EncodeToString(secret[:])}
	digest := sha256.Sum256(secret[:])
	err = fixture.pool.QueryRow(ctx, `INSERT INTO directory.site_audits
		(lookup_secret_hash, action, proposed_snapshot, request_reason, submitter_email, notify_by_email)
		VALUES ($1, 'CREATE', $2, 'Historical request', 'historical@example.test', true)
		RETURNING id::text`, digest[:], encoded).Scan(&result.AuditID)
	if err != nil {
		t.Fatalf("insert historical audit: %v", err)
	}
	return result
}

func (fixture auditMigrationFixture) addReviewHistory(t *testing.T, auditID string) {
	t.Helper()
	ctx := t.Context()
	siteID := insertSite(ctx, t, fixture.pool, "4Fjw77W5U", "Historical approved site", "historical-approved.example.test")
	reviewer, err := dbgen.New(fixture.pool).CreateUser(ctx, dbgen.CreateUserParams{
		Email: "historical-reviewer@example.test", Username: "historical_reviewer", DisplayName: "Historical reviewer",
	})
	if err != nil {
		t.Fatalf("create historical reviewer: %v", err)
	}
	_, err = fixture.pool.Exec(ctx, `UPDATE directory.site_audits
		SET review_draft_snapshot = proposed_snapshot, review_draft_revision = 1,
		    review_draft_updated_by = $2, review_draft_updated_at = now() WHERE id = $1`, auditID, reviewer.ID)
	if err != nil {
		t.Fatalf("seed historical review draft: %v", err)
	}
	_, err = fixture.pool.Exec(ctx, `INSERT INTO directory.site_audits
		(lookup_secret_hash, action, status, site_id, base_revision, base_snapshot, proposed_snapshot,
		 final_snapshot, request_reason, reviewed_by, reviewed_at)
		SELECT decode(repeat('ab', 32), 'hex'), 'UPDATE', 'APPROVED', $2, 1,
		       proposed_snapshot, proposed_snapshot, proposed_snapshot, 'Historical approval', $3, now()
		FROM directory.site_audits WHERE id = $1`, auditID, siteID, reviewer.ID)
	if err != nil {
		t.Fatalf("seed historical approved snapshots: %v", err)
	}
}

func (fixture auditMigrationFixture) evidenceOutsideTaxonomy(t *testing.T) string {
	t.Helper()
	var evidence string
	err := fixture.pool.QueryRow(t.Context(), `SELECT jsonb_agg(
		(to_jsonb(audit) - ARRAY['base_snapshot', 'proposed_snapshot', 'review_draft_snapshot', 'final_snapshot'])
		|| jsonb_build_object(
		    'base_snapshot', base_snapshot - ARRAY['tag_cascade_id', 'classification', 'tags'],
		    'proposed_snapshot', proposed_snapshot - ARRAY['tag_cascade_id', 'classification', 'tags'],
		    'review_draft_snapshot', review_draft_snapshot - ARRAY['tag_cascade_id', 'classification', 'tags'],
		    'final_snapshot', final_snapshot - ARRAY['tag_cascade_id', 'classification', 'tags']
		) ORDER BY id)::text FROM directory.site_audits AS audit`).Scan(&evidence)
	if err != nil {
		t.Fatalf("read historical evidence outside taxonomy: %v", err)
	}
	return evidence
}
