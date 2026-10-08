//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"heyblog-api/internal/features/siteaudit"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func verifyFreshOwnerFriend(ctx context.Context, t *testing.T, pool *pgxpool.Pool, fixture ownerWorkflowFixture) {
	t.Helper()
	input := fixture.input
	input.Site.URL = "https://owner-fresh-friend.example.test/"
	result, err := fixture.accounts.Submit(ctx, fixture.user, "Owner0001", "OWNER_FRIEND_LINK", input)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fixture.audits.Review(ctx, fixture.reviewer, siteaudit.ReviewInput{AuditID: result.AuditID, Decision: siteaudit.DecisionApprove})
	if err != nil {
		t.Fatal(err)
	}
	var recommendations int
	if err := pool.QueryRow(ctx, `SELECT jsonb_array_length(o.metadata->'recommendations')
	FROM directory.site_origins o JOIN directory.site_sources s ON s.id=o.source_id JOIN directory.sites site ON site.id=o.site_id
	WHERE site.normalized_host='owner-fresh-friend.example.test' AND s.source_key='OWNER_FRIEND_LINK'`).Scan(&recommendations); err != nil {
		t.Fatal(err)
	}
	if recommendations != 1 {
		t.Fatalf("fresh friend origin recommendations = %d", recommendations)
	}
}

func verifyOwnerAnonymization(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	var userID, auditID, ownershipID pgtype.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO identity.users(email,username,display_name,access_status,email_verified_at,
	deletion_requested_at,deletion_scheduled_for,created_at) VALUES('private-owner@example.test','private_owner','Private owner','SUSPENDED',
	now()-interval '31 days',now()-interval '31 days',now()-interval '1 day',now()-interval '32 days') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	siteID := insertSite(ctx, t, pool, "Privacy01", "Private ownership", "private-owner.example.test")
	if err := pool.QueryRow(ctx, `INSERT INTO directory.site_ownerships(site_id,user_id,address) VALUES($1,$2,'https://private-owner.example.test/') RETURNING id`, siteID, userID).Scan(&ownershipID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO directory.site_audits(lookup_secret_hash,action,proposed_snapshot,request_reason,
	submitter_user_id,source_channel,submitter_name,submitter_email,notify_by_email)
	VALUES(decode(repeat('ed',32),'hex'),'CREATE','{"name":"History","scheme":"https","normalized_host":"history.example.test","base_path":"/"}',
	'Historical submission',$1,'ACCOUNT_SUBMISSION','Private owner','private-owner@example.test',true) RETURNING id`, userID).Scan(&auditID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO directory.owner_friend_link_requests(audit_id,source_site_id,user_id,ownership_id) VALUES($1,$2,$3,$4)`, auditID, siteID, userID, ownershipID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO directory.site_sources(source_key,name) VALUES('PRIVACY_TEST','History test');
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO directory.site_origins(site_id,source_id,metadata) SELECT $1,id,
	jsonb_build_object('user_id',$2::text,'recommendations',jsonb_build_array(jsonb_build_object('user_id',$2::text,'source_site_id',$1::uuid::text)))
	FROM directory.site_sources WHERE source_key='PRIVACY_TEST'`, siteID, integrationUUIDText(t, userID)); err != nil {
		t.Fatal(err)
	}
	if _, err := dbgen.New(pool).CompleteUserDeletion(ctx, userID); err != nil {
		t.Fatal(err)
	}
	var anonymousAudit, anonymousRequest, anonymousOrigin bool
	if err := pool.QueryRow(ctx, `SELECT submitter_user_id IS NULL AND submitter_name IS NULL AND submitter_email IS NULL AND NOT notify_by_email FROM directory.site_audits WHERE id=$1`, auditID).Scan(&anonymousAudit); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT user_id IS NULL AND status='CANCELLED' FROM directory.owner_friend_link_requests WHERE audit_id=$1`, auditID).Scan(&anonymousRequest); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT NOT metadata ? 'user_id' AND NOT (metadata->'recommendations'->0) ? 'user_id' FROM directory.site_origins WHERE site_id=$1`, siteID).Scan(&anonymousOrigin); err != nil {
		t.Fatal(err)
	}
	if !anonymousAudit || !anonymousRequest || !anonymousOrigin {
		t.Fatalf("history anonymization = audit:%v request:%v origin:%v", anonymousAudit, anonymousRequest, anonymousOrigin)
	}
}
