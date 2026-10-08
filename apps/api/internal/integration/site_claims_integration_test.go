//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/sitemanagement"
)

type claimsProofVerifier struct{}

func (claimsProofVerifier) Verify(context.Context, sitemanagement.Claim) error { return nil }

func verifySiteClaims(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	var users []auth.User
	for i := 0; i < 3; i++ {
		var id string
		err := pool.QueryRow(ctx, `INSERT INTO identity.users(email,username,display_name) VALUES($1,$2,$2) RETURNING id::text`, fmt.Sprintf("claims%d@example.test", i), fmt.Sprintf("claims_user_%d", i)).Scan(&id)
		require.NoError(t, err)
		users = append(users, auth.User{ID: id, Role: auth.RoleUser})
	}
	reviewer := users[2]
	reviewer.Role = auth.RoleSysAdmin
	repository := sitemanagement.NewRepository(pool)
	service := sitemanagement.NewService(repository, nil, claimsProofVerifier{})
	t.Run("proof expiring while waiting for the site lock cannot grant ownership", func(t *testing.T) {
		// Given a valid challenge whose decision is blocked behind a site update.
		siteID := insertSite(ctx, t, pool, "Claim0003", "Expiring claim", "claim-expiring.example.com")
		result, err := service.Create(ctx, users[0], sitemanagement.SiteClaimCreateInput{ShortID: "Claim0003", Method: sitemanagement.File})
		require.NoError(t, err)
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()
		_, err = tx.Exec(ctx, `SELECT id FROM directory.sites WHERE id=$1 FOR UPDATE`, siteID)
		require.NoError(t, err)
		finished := make(chan error, 1)
		go func() {
			_, err := repository.Decide(ctx, sitemanagement.Decision{Claim: result.Claim, Status: "VERIFIED", Now: time.Now()})
			finished <- err
		}()
		require.Eventually(t, func() bool {
			var waiting bool
			err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE '%SELECT id::text, short_id,%')`).Scan(&waiting)
			return err == nil && waiting
		}, 5*time.Second, 10*time.Millisecond)
		// When the proof expires before the lock is released.
		_, err = tx.Exec(ctx, `UPDATE directory.site_claims SET expires_at=clock_timestamp()-interval '1 millisecond' WHERE id=$1::uuid`, result.Claim.ID)
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))
		require.ErrorContains(t, <-finished, "expired")
		// Then the claim, ownership and event changes all remain unapplied.
		var count int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM directory.site_ownerships WHERE site_id=$1`, siteID).Scan(&count))
		require.Zero(t, count)
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM directory.site_ownership_events WHERE site_id=$1`, siteID).Scan(&count))
		require.Zero(t, count)
		claim, err := repository.Claim(ctx, result.Claim.ID)
		require.NoError(t, err)
		require.Equal(t, "PENDING", claim.Status)
	})
	t.Run("concurrent verification has one owner", func(t *testing.T) {
		// Given two applicants with independently issued proofs.
		insertSite(ctx, t, pool, "Claim0001", "Claim concurrency", "claim-concurrency.example.com")
		claims := make([]sitemanagement.CreateResult, 2)
		for i := 0; i < 2; i++ {
			result, err := service.Create(ctx, users[i], sitemanagement.SiteClaimCreateInput{ShortID: "Claim0001", Method: sitemanagement.DNS})
			require.NoError(t, err)
			claims[i] = result
		}
		// When they concurrently verify the same registered site.
		var group sync.WaitGroup
		results := make(chan error, 2)
		for i := 0; i < 2; i++ {
			group.Go(func() { _, err := service.Check(ctx, users[i], claims[i].Claim.ID); results <- err })
		}
		group.Wait()
		close(results)
		// Then exactly one ownership generation wins.
		successes := 0
		for err := range results {
			if err == nil {
				successes++
			}
		}
		require.Equal(t, 1, successes)
		var count int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM directory.site_ownerships o JOIN directory.sites s ON s.id=o.site_id WHERE s.short_id='Claim0001'`).Scan(&count))
		require.Equal(t, 1, count)
	})
	t.Run("manual review establishes ownership and rejects self review", func(t *testing.T) {
		// Given an evidence-backed manual application.
		insertSite(ctx, t, pool, "Claim0002", "Manual claim", "claim-manual.example.com")
		result, err := service.Create(ctx, users[0], sitemanagement.SiteClaimCreateInput{ShortID: "Claim0002", Method: sitemanagement.Manual, Evidence: "Public domain registration and author statement", EvidenceURL: "https://claim-manual.example.com/about"})
		require.NoError(t, err)
		self := users[0]
		self.Role = auth.RoleSysAdmin
		_, err = service.Review(ctx, self, result.Claim.ID, sitemanagement.SiteClaimReviewInput{Approve: true, Reason: "Self proof"})
		require.Error(t, err)
		// When an authorized independent reviewer approves.
		approved, err := service.Review(ctx, reviewer, result.Claim.ID, sitemanagement.SiteClaimReviewInput{Approve: true, Reason: "Public evidence checked"})
		// Then the owner can edit and the application cannot be replayed.
		require.NoError(t, err)
		require.Equal(t, "VERIFIED", approved.Status)
		_, err = service.CurrentOwner(ctx, users[0], "Claim0002")
		require.NoError(t, err)
		_, err = service.Review(ctx, reviewer, result.Claim.ID, sitemanagement.SiteClaimReviewInput{Approve: true, Reason: "Again"})
		require.Error(t, err)
	})
	t.Run("address proof consumption is atomic and revocation invalidates ownership generation", func(t *testing.T) {
		// Given an owner and separately verified new-address proof.
		owner, err := service.CurrentOwner(ctx, users[0], "Claim0002")
		require.NoError(t, err)
		proof, err := service.Create(ctx, users[0], sitemanagement.SiteClaimCreateInput{ShortID: "Claim0002", Method: sitemanagement.File, Address: "https://claim-new.example.com/blog/"})
		require.NoError(t, err)
		_, err = service.Check(ctx, users[0], proof.Claim.ID)
		require.NoError(t, err)
		var current string
		require.NoError(t, pool.QueryRow(ctx, `SELECT address FROM directory.site_ownerships WHERE id=$1::uuid`, owner.ID).Scan(&current))
		require.Equal(t, owner.Address, current)
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()
		_, err = tx.Exec(ctx, `SELECT directory.apply_verified_site_address($1::uuid,$2::uuid,$3,$4::uuid)`, owner.SiteID, users[0].ID, proof.Claim.Address, owner.ID)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, `UPDATE directory.sites SET scheme='https',normalized_host='claim-new.example.com',base_path='/blog' WHERE id=$1::uuid`, owner.SiteID)
		require.NoError(t, err)
		// When the complete address transition commits.
		require.NoError(t, tx.Commit(ctx))
		// Then the proof is consumed and the new address is authoritative.
		refreshed, err := service.CurrentOwner(ctx, users[0], "Claim0002")
		require.NoError(t, err)
		require.Equal(t, proof.Claim.Address, refreshed.Address)
		var consumed bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT consumed_at IS NOT NULL FROM directory.site_claims WHERE id=$1::uuid`, proof.Claim.ID).Scan(&consumed))
		require.True(t, consumed)
		err = service.ChangeOwner(ctx, reviewer, "Claim0002", sitemanagement.OwnershipInput{UserID: users[1].ID, Reason: "Ownership transferred", Evidence: "Independent public evidence"})
		require.NoError(t, err)
		newOwner, err := service.CurrentOwner(ctx, users[1], "Claim0002")
		require.NoError(t, err)
		require.NotEqual(t, owner.ID, newOwner.ID)
		_, err = pool.Exec(ctx, `SELECT directory.apply_verified_site_address($1::uuid,$2::uuid,$3,$4::uuid)`, owner.SiteID, users[0].ID, proof.Claim.Address, owner.ID)
		require.Error(t, err)
		require.NoError(t, service.ChangeOwner(ctx, reviewer, "Claim0002", sitemanagement.OwnershipInput{Reason: "Ownership revoked", Evidence: "Independent revocation evidence"}))
		var revokedUser string
		require.NoError(t, pool.QueryRow(ctx, `SELECT coalesce(user_id::text,'') FROM directory.site_ownership_events WHERE site_id=$1::uuid AND action='REVOKED'`, owner.SiteID).Scan(&revokedUser))
		require.Equal(t, users[1].ID, revokedUser)
	})
}
