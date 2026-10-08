//go:build integration

package integration_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"heyblog-api/internal/domain/site"
	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/siteaudit"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"heyblog-api/internal/infrastructure/mail"
)

type ownerWorkflowFixture struct {
	accounts *siteaudit.AccountService
	audits   *siteaudit.Service
	user     auth.User
	reviewer auth.User
	siteID   pgtype.UUID
	input    siteaudit.SubmissionInput
	mail     *authMailRecorder
}

func verifyOwnerSiteWorkflows(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	fixture := newOwnerWorkflowFixture(ctx, t, pool)
	items, err := fixture.accounts.Sites(ctx, fixture.user)
	if err != nil || len(items) != 1 || items[0].ShortID != "Owner0001" {
		t.Fatalf("account sites = %#v / %v", items, err)
	}
	outsider := fixture.reviewer
	if _, err := fixture.accounts.Site(ctx, outsider, "Owner0001"); err == nil {
		t.Fatal("another account read the owned site")
	}
	if err := fixture.accounts.SetFriend(ctx, outsider, "Owner0001", "Friend001", true); err == nil {
		t.Fatal("another account changed friends")
	}
	if err := fixture.accounts.SetFriend(ctx, fixture.user, "Owner0001", "Owner0001", true); err == nil {
		t.Fatal("self friend link accepted")
	}
	if err := fixture.accounts.SetFriend(ctx, fixture.user, "Owner0001", "Friend001", true); err != nil {
		t.Fatal(err)
	}
	friends, err := fixture.accounts.Friends(ctx, fixture.user, "Owner0001")
	if err != nil || len(friends.Items) != 1 || friends.Items[0].LinkStatus != "ACTIVE" {
		t.Fatalf("active friend = %#v / %v", friends, err)
	}
	if err := fixture.accounts.SetFriend(ctx, fixture.user, "Owner0001", "Friend001", false); err != nil {
		t.Fatal(err)
	}
	friends, err = fixture.accounts.Friends(ctx, fixture.user, "Owner0001")
	if err != nil || friends.Items[0].LinkStatus != "INACTIVE" {
		t.Fatalf("removed friend = %#v / %v", friends, err)
	}
	t.Run("existing external friend links can be removed only by their owner", func(t *testing.T) {
		_, err := pool.Exec(ctx, `UPDATE directory.sites SET visibility='HIDDEN' WHERE id=$1`, fixture.siteID)
		require.NoError(t, err)
		for _, host := range []string{"owner-external-one.example.test", "owner-external-two.example.test"} {
			if _, err := pool.Exec(ctx, `SELECT directory.upsert_friend_link($1,$2,$3,'ACTIVE')`, fixture.siteID, "https://"+host+"/friends", host); err != nil {
				t.Fatal(err)
			}
		}
		if err := fixture.accounts.RemoveFriend(ctx, outsider, "Owner0001", "owner-external-one.example.test"); err == nil {
			t.Fatal("another account removed an external friend")
		}
		if err := fixture.accounts.RemoveFriend(ctx, fixture.user, "Owner0001", "owner-external-one.example.test"); err != nil {
			t.Fatal(err)
		}
		if err := fixture.accounts.RemoveFriend(ctx, fixture.user, "Owner0001", "missing.example.test"); err == nil {
			t.Fatal("missing friend link accepted")
		}
		friends, err := fixture.accounts.Friends(ctx, fixture.user, "Owner0001")
		if err != nil {
			t.Fatal(err)
		}
		statuses := map[string]string{}
		for _, friend := range friends.Items {
			statuses[friend.TargetHost] = friend.LinkStatus
		}
		if statuses["owner-external-one.example.test"] != "INACTIVE" || statuses["owner-external-two.example.test"] != "ACTIVE" {
			t.Fatalf("external friends = %#v", statuses)
		}
		_, err = pool.Exec(ctx, `UPDATE directory.sites SET visibility='VISIBLE' WHERE id=$1`, fixture.siteID)
		require.NoError(t, err)
	})
	t.Run("friend removal and target address updates share a lock order", func(t *testing.T) {
		// Given an active edge and a target address transaction holding its graph reference.
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		require.NoError(t, fixture.accounts.SetFriend(ctx, fixture.user, "Owner0001", "Friend001", true))
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()
		_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('site-ref:owner-friend.example.test',0))`)
		require.NoError(t, err)
		finished := make(chan error, 1)
		go func() {
			finished <- fixture.accounts.RemoveFriend(ctx, fixture.user, "Owner0001", "owner-friend.example.test")
		}()
		require.Eventually(t, func() bool {
			var waiting bool
			err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE '%SELECT directory.remove_owner_friend_link%')`).Scan(&waiting)
			return err == nil && waiting
		}, 3*time.Second, 10*time.Millisecond)
		// When the target moves while removal waits, neither transaction deadlocks.
		_, err = tx.Exec(ctx, `UPDATE directory.sites SET base_path='/moved' WHERE short_id='Friend001'`)
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))
		require.NoError(t, <-finished)
		friends, err := fixture.accounts.Friends(ctx, fixture.user, "Owner0001")
		require.NoError(t, err)
		for _, friend := range friends.Items {
			if friend.TargetHost == "owner-friend.example.test" {
				require.Equal(t, "INACTIVE", friend.LinkStatus)
				require.Equal(t, "https://owner-friend.example.test/moved", friend.TargetURL)
			}
		}
	})

	input := fixture.input
	input.Site.URL = "https://owner-source.example.test/"
	input.Site.Name = "Owner edited name"
	updated, err := fixture.accounts.Submit(ctx, fixture.user, "Owner0001", "OWNER_UPDATE", input)
	if err != nil || updated.LookupToken != "" {
		t.Fatalf("owner submission = %#v / %v", updated, err)
	}
	before, err := fixture.accounts.Site(ctx, fixture.user, "Owner0001")
	if err != nil || before.Name == input.Site.Name {
		t.Fatalf("pending edit changed canonical site = %#v / %v", before, err)
	}
	if _, err := fixture.accounts.Submission(ctx, outsider, updated.AuditID); err == nil {
		t.Fatal("another account read submission")
	}
	approved, err := fixture.audits.Review(ctx, fixture.reviewer, siteaudit.ReviewInput{AuditID: updated.AuditID, Decision: siteaudit.DecisionApprove, ExpectedSiteRevision: before.Revision})
	if err != nil || approved.FinalSnapshot.Name != input.Site.Name || approved.SourceChannel != "OWNER_UPDATE" {
		t.Fatalf("approve owner edit = %#v / %v", approved, err)
	}
	after, err := fixture.accounts.Site(ctx, fixture.user, "Owner0001")
	require.NoError(t, err)
	require.Equal(t, after.Revision, approved.FinalSnapshot.Revision, "approved snapshot must match the canonical site revision")

	verifyOwnerAddressChange(ctx, t, pool, fixture)
	verifyOwnerFriendApproval(ctx, t, pool, fixture)
	verifyOwnerGenerationRollback(ctx, t, pool, fixture)
	verifyOwnerAnonymization(ctx, t, pool)
}

func newOwnerWorkflowFixture(ctx context.Context, t *testing.T, pool *pgxpool.Pool) ownerWorkflowFixture {
	t.Helper()
	queries := dbgen.New(pool)
	user, err := queries.CreateUser(ctx, dbgen.CreateUserParams{Email: "owner-workflow@example.test", Username: "owner_workflow", DisplayName: "Owner workflow"})
	if err != nil {
		t.Fatal(err)
	}
	reviewer, err := queries.CreateUser(ctx, dbgen.CreateUserParams{Email: "owner-reviewer@example.test", Username: "owner_reviewer", DisplayName: "Owner reviewer"})
	if err != nil {
		t.Fatal(err)
	}
	siteID := insertSite(ctx, t, pool, "Owner0001", "Owner source", "owner-source.example.test")
	insertSite(ctx, t, pool, "Friend001", "Registered friend", "owner-friend.example.test")
	if _, err := pool.Exec(ctx, `INSERT INTO directory.site_ownerships(site_id,user_id,address) VALUES($1,$2,'https://owner-source.example.test/')`, siteID, user.ID); err != nil {
		t.Fatal(err)
	}
	cascades, err := queries.ListEnabledSiteTagCascades(ctx)
	if err != nil || len(cascades) == 0 {
		t.Fatalf("owner taxonomy: %v", err)
	}
	program, err := queries.GetSoftwareComponentByNormalizedName(ctx, "其他")
	if err != nil {
		t.Fatal(err)
	}
	input := siteauditConflictSubmission("https://owner-new-friend.example.test/", siteauditConflictTaxonomy{Level1ID: integrationUUIDText(t, cascades[0].Level1ID), Level2ID: integrationUUIDText(t, cascades[0].Level2ID)}, integrationUUIDText(t, program.ID))
	input.Reason = "Owner requested change"
	repository := siteaudit.NewRepository(pool)
	recorder := &authMailRecorder{}
	audits := siteaudit.NewService(siteaudit.Dependencies{Repository: repository, NewShortID: site.NewShortID, Mailer: mail.NewSubmissionMailer(recorder, "audit@example.test")})
	email := "owner-workflow@example.test"
	return ownerWorkflowFixture{accounts: siteaudit.NewAccountService(repository, audits), audits: audits,
		user:     auth.User{ID: integrationUUIDText(t, user.ID), DisplayName: "Owner workflow", Email: &email},
		reviewer: auth.User{ID: integrationUUIDText(t, reviewer.ID), Role: auth.RoleSysAdmin}, siteID: siteID, input: input, mail: recorder}
}

func verifyOwnerAddressChange(ctx context.Context, t *testing.T, pool *pgxpool.Pool, fixture ownerWorkflowFixture) {
	t.Helper()
	input := fixture.input
	input.Site.URL = "https://owner-moved.example.test/blog"
	if _, err := fixture.accounts.Submit(ctx, fixture.user, "Owner0001", "OWNER_UPDATE", input); err == nil {
		t.Fatal("unverified new address accepted")
	} else {
		var serviceError *siteaudit.ServiceError
		if !errors.As(err, &serviceError) || serviceError.Code != "verified_site_address_required" {
			t.Fatalf("unverified address error = %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO directory.site_claims(site_id,user_id,address,method,status,evidence,verified_at)
	VALUES($1,$2,$3,'MANUAL','VERIFIED','Public proof',now())`, fixture.siteID, fixture.user.ID, input.Site.URL); err != nil {
		t.Fatal(err)
	}
	result, err := fixture.accounts.Submit(ctx, fixture.user, "Owner0001", "OWNER_UPDATE", input)
	if err != nil {
		t.Fatal(err)
	}
	before, err := fixture.accounts.Site(ctx, fixture.user, "Owner0001")
	if err != nil || before.NormalizedHost != "owner-source.example.test" {
		t.Fatalf("new address changed before approval: %#v / %v", before, err)
	}
	_, err = fixture.audits.Review(ctx, fixture.reviewer, siteaudit.ReviewInput{AuditID: result.AuditID, Decision: siteaudit.DecisionApprove, ExpectedSiteRevision: before.Revision})
	if err != nil {
		t.Fatal(err)
	}
	var address string
	var consumed bool
	if err := pool.QueryRow(ctx, `SELECT o.address, c.consumed_at IS NOT NULL FROM directory.site_ownerships o JOIN directory.site_claims c ON c.id=o.verified_claim_id WHERE o.site_id=$1`, fixture.siteID).Scan(&address, &consumed); err != nil {
		t.Fatal(err)
	}
	if address != input.Site.URL || !consumed {
		t.Fatalf("address and proof not switched atomically: %s / %v", address, consumed)
	}
}

func verifyOwnerFriendApproval(ctx context.Context, t *testing.T, pool *pgxpool.Pool, fixture ownerWorkflowFixture) {
	t.Helper()
	// An anonymous pending site can receive an owner's recommendation without leaking its lookup secret.
	input := fixture.input
	input.Contact = siteaudit.ContactInput{Name: "Private submitter", Email: "private@example.test"}
	anonymous, err := fixture.audits.Submit(ctx, siteaudit.ActionCreate, "", input)
	if err != nil {
		t.Fatal(err)
	}
	ownerInput := fixture.input
	ownerInput.Contact = siteaudit.ContactInput{NotifyByEmail: true}
	fixture.mail.messages = nil
	result, err := fixture.accounts.Submit(ctx, fixture.user, "Owner0001", "OWNER_FRIEND_LINK", ownerInput)
	if err != nil || result.AuditID != anonymous.AuditID || result.RequestID == "" || result.LookupToken != "" {
		t.Fatalf("shared target = %#v / %v", result, err)
	}
	t.Run("older pending friends survive a full page of finished history", func(t *testing.T) {
		source := insertSite(ctx, t, pool, "OwnPage01", "Friend history source", "owner-history-page.example.test")
		var ownership string
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO directory.site_ownerships(site_id,user_id,address) VALUES($1,$2::uuid,'https://owner-history-page.example.test/') RETURNING id::text`, source, fixture.user.ID).Scan(&ownership))
		var pending string
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO directory.owner_friend_link_requests(audit_id,source_site_id,user_id,ownership_id,created_at) VALUES($1::uuid,$2,$3::uuid,$4::uuid,now()-interval '1 hour') RETURNING id::text`, result.AuditID, source, fixture.user.ID, ownership).Scan(&pending))
		_, err := pool.Exec(ctx, `INSERT INTO directory.owner_friend_link_requests(audit_id,source_site_id,user_id,ownership_id,status) SELECT $1::uuid,$2,$3::uuid,uuidv7(),'CANCELLED' FROM generate_series(1,51)`, result.AuditID, source, fixture.user.ID)
		require.NoError(t, err)
		friends, err := fixture.accounts.Friends(ctx, fixture.user, "OwnPage01")
		require.NoError(t, err)
		require.Len(t, friends.Pending, 51)
		found := false
		for _, request := range friends.Pending {
			if request.ID == pending && request.Status == "PENDING" {
				found = true
			}
		}
		require.True(t, found, "pending request was hidden by newer finished history")
	})
	detail, err := fixture.accounts.Submission(ctx, fixture.user, result.AuditID)
	if err != nil || detail.SubmitterEmail != "" || detail.SubmitterName != "" {
		t.Fatalf("shared target leaked contact: %#v / %v", detail, err)
	}
	_, err = fixture.audits.Review(ctx, fixture.reviewer, siteaudit.ReviewInput{AuditID: result.AuditID, Decision: siteaudit.DecisionApprove})
	if err != nil {
		t.Fatal(err)
	}
	if len(fixture.mail.messages) != 1 || fixture.mail.messages[0].To != *fixture.user.Email || !strings.Contains(fixture.mail.messages[0].Text, "我的提交") {
		t.Fatalf("shared recommendation notification = %#v", fixture.mail.messages)
	}
	friends, err := fixture.accounts.Friends(ctx, fixture.user, "Owner0001")
	if err != nil || len(friends.Pending) != 1 || friends.Pending[0].Status != "APPLIED" {
		t.Fatalf("pending recommendation not applied = %#v / %v", friends, err)
	}
	var originCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM directory.site_origins o JOIN directory.site_sources s ON s.id=o.source_id JOIN directory.sites site ON site.id=o.site_id
WHERE site.normalized_host='owner-new-friend.example.test' AND s.source_key='OWNER_FRIEND_LINK' AND o.metadata->'recommendations'->0->>'source_site_id'=$1`, integrationUUIDText(t, fixture.siteID)).Scan(&originCount); err != nil {
		t.Fatal(err)
	}
	if originCount != 1 {
		t.Fatalf("friend discovery origin count = %d", originCount)
	}
	active := false
	for _, friend := range friends.Items {
		if friend.TargetHost == "owner-new-friend.example.test" && friend.LinkStatus == "ACTIVE" {
			active = true
		}
	}
	if !active {
		t.Fatal("approval did not create graph link")
	}
	verifyFreshOwnerFriend(ctx, t, pool, fixture)

	input = fixture.input
	input.Site.URL = "https://owner-cancelled-friend.example.test/"
	cancelled, err := fixture.accounts.Submit(ctx, fixture.user, "Owner0001", "OWNER_FRIEND_LINK", input)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.accounts.CancelFriend(ctx, fixture.user, "Owner0001", cancelled.RequestID); err != nil {
		t.Fatal(err)
	}
	_, err = fixture.audits.Review(ctx, fixture.reviewer, siteaudit.ReviewInput{AuditID: cancelled.AuditID, Decision: siteaudit.DecisionReject, ReviewerComment: "Rejected target"})
	if err != nil {
		t.Fatal(err)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM directory.owner_friend_link_requests WHERE id=$1`, cancelled.RequestID).Scan(&status); err != nil || status != "CANCELLED" {
		t.Fatalf("cancelled request status = %s / %v", status, err)
	}
}

func verifyOwnerGenerationRollback(ctx context.Context, t *testing.T, pool *pgxpool.Pool, fixture ownerWorkflowFixture) {
	t.Helper()
	input := fixture.input
	input.Site.URL = "https://owner-moved.example.test/blog"
	input.Site.Name = "Stale owner edit"
	result, err := fixture.accounts.Submit(ctx, fixture.user, "Owner0001", "OWNER_UPDATE", input)
	if err != nil {
		t.Fatal(err)
	}
	before, err := fixture.accounts.Site(ctx, fixture.user, "Owner0001")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM directory.site_ownerships WHERE site_id=$1`, fixture.siteID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO directory.site_ownerships(site_id,user_id,address) VALUES($1,$2,$3)`, fixture.siteID, fixture.user.ID, input.Site.URL); err != nil {
		t.Fatal(err)
	}
	_, err = fixture.audits.Review(ctx, fixture.reviewer, siteaudit.ReviewInput{AuditID: result.AuditID, Decision: siteaudit.DecisionApprove, ExpectedSiteRevision: before.Revision})
	if err == nil {
		t.Fatal("stale ownership generation approved")
	}
	after, err := fixture.accounts.Site(ctx, fixture.user, "Owner0001")
	if err != nil || after.Name != before.Name || after.Revision != before.Revision {
		t.Fatalf("stale approval did not roll back: %#v / %v", after, err)
	}
	assertSiteAuditStatus(t, ctx, pool, result.AuditID, siteaudit.StatusPending)
}
