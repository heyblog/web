//go:build integration

package integration_test

import (
	"context"
	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/sitemanagement"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type waitingClaimVerifier struct{ entered, release chan struct{} }

func (v waitingClaimVerifier) Verify(ctx context.Context, _ sitemanagement.Claim) error {
	close(v.entered)
	select {
	case <-v.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func verifySiteClaimRecovery(ctx context.Context, t *testing.T, pool *pgxpool.Pool, users []auth.User, reviewer auth.User) {
	t.Helper()
	repository := sitemanagement.NewRepository(pool)
	service := sitemanagement.NewService(repository, nil, claimsProofVerifier{})
	t.Run("duplicate and expired proof recovery retain one application and reject stale network success", func(t *testing.T) {
		siteID := insertSite(ctx, t, pool, "Claim0004", "Proof recovery", "claim-recovery.example.com")
		input := sitemanagement.SiteClaimCreateInput{ShortID: "Claim0004", Method: sitemanagement.File}
		first, err := service.Create(ctx, users[0], input)
		require.NoError(t, err)
		again, err := service.Create(ctx, users[0], input)
		require.NoError(t, err)
		require.Equal(t, first.Claim.ID, again.Claim.ID)
		require.Empty(t, again.Token)
		require.Nil(t, again.Instructions)
		_, err = pool.Exec(ctx, `UPDATE directory.site_claims SET created_at=clock_timestamp()-interval '2 days',expires_at=clock_timestamp()-interval '1 minute' WHERE id=$1::uuid`, first.Claim.ID)
		require.NoError(t, err)
		again, err = service.Create(ctx, users[0], input)
		require.NoError(t, err)
		require.Equal(t, first.Claim.ID, again.Claim.ID)
		require.Empty(t, again.Token)
		require.False(t, again.Claim.CanCheck)
		require.True(t, again.Claim.CanRegenerate)
		regenerated, err := service.Regenerate(ctx, users[0], first.Claim.ID)
		require.NoError(t, err)
		require.NotEqual(t, first.Token, regenerated.Token)
		require.Equal(t, first.Claim.ID, regenerated.Claim.ID)
		_, err = service.Detail(ctx, users[1], first.Claim.ID, false)
		require.ErrorIs(t, err, sitemanagement.ErrNotFound)
		_, err = service.Regenerate(ctx, users[1], first.Claim.ID)
		require.ErrorIs(t, err, sitemanagement.ErrNotFound)
		verifier := waitingClaimVerifier{entered: make(chan struct{}), release: make(chan struct{})}
		checking := sitemanagement.NewService(repository, nil, verifier)
		finished := make(chan error, 1)
		go func() { _, err := checking.Check(ctx, users[0], first.Claim.ID); finished <- err }()
		<-verifier.entered
		newest, err := service.Regenerate(ctx, users[0], first.Claim.ID)
		require.NoError(t, err)
		close(verifier.release)
		var changed *sitemanagement.ServiceError
		require.ErrorAs(t, <-finished, &changed)
		require.Equal(t, "proof_changed", changed.Code)
		_, err = repository.Owner(ctx, first.Claim.SiteID)
		require.ErrorIs(t, err, sitemanagement.ErrNotFound)
		_, err = service.Create(ctx, users[0], sitemanagement.SiteClaimCreateInput{ShortID: "Claim0004", Method: sitemanagement.DNS})
		require.NoError(t, err)
		own, err := service.List(ctx, users[0], sitemanagement.ClaimFilter{Limit: 1}, false)
		require.NoError(t, err)
		require.True(t, own.HasMore)
		require.Nil(t, own.Items[0].Applicant)
		managed, err := service.List(ctx, reviewer, sitemanagement.ClaimFilter{Limit: 1, Status: "PENDING", Method: "FILE"}, true)
		require.NoError(t, err)
		require.Len(t, managed.Items, 1)
		require.NotNil(t, managed.Items[0].Applicant)
		verified, err := service.Check(ctx, users[0], newest.Claim.ID)
		require.NoError(t, err)
		require.NotNil(t, verified.VerifiedAt)
		require.NotNil(t, verified.ConsumedAt)
		require.True(t, verified.CanManage)
		detail, err := service.Context(ctx, users[0], "Claim0004")
		require.NoError(t, err)
		require.Equal(t, "mine", detail.OwnershipState)
		require.True(t, detail.CanManage)
		require.False(t, detail.CanClaim)
		other, err := service.Context(ctx, users[1], "Claim0004")
		require.NoError(t, err)
		require.Equal(t, "other", other.OwnershipState)
		require.Empty(t, other.PendingClaims)
		_, err = pool.Exec(ctx, `UPDATE directory.sites SET base_path='/changed' WHERE id=$1::uuid`, siteID)
		require.NoError(t, err)
		detail, err = service.Context(ctx, users[0], "Claim0004")
		require.NoError(t, err)
		require.Equal(t, "stale", detail.OwnershipState)
		require.False(t, detail.CanManage)
		require.True(t, detail.CanVerifyAddress)
	})
	t.Run("revocation cancels an older manual application before it can restore ownership", func(t *testing.T) {
		insertSite(ctx, t, pool, "Claim0005", "Revocation recovery", "claim-revocation.example.com")
		pending, err := service.Create(ctx, users[0], sitemanagement.SiteClaimCreateInput{ShortID: "Claim0005", Method: sitemanagement.Manual, Evidence: "Author ownership statement"})
		require.NoError(t, err)
		proof, err := service.Create(ctx, users[0], sitemanagement.SiteClaimCreateInput{ShortID: "Claim0005", Method: sitemanagement.File})
		require.NoError(t, err)
		_, err = service.Check(ctx, users[0], proof.Claim.ID)
		require.NoError(t, err)
		owner, err := service.CurrentOwner(ctx, users[0], "Claim0005")
		require.NoError(t, err)
		revoke := sitemanagement.OwnershipInput{OwnershipID: owner.ID, Revision: owner.Revision, Reason: "Ownership revoked", Evidence: "Domain control changed"}
		self := users[0]
		self.Role = auth.RoleSysAdmin
		require.Error(t, service.Revoke(ctx, self, "Claim0005", revoke))
		require.NoError(t, service.Revoke(ctx, reviewer, "Claim0005", revoke))
		_, err = service.Review(ctx, reviewer, pending.Claim.ID, sitemanagement.SiteClaimReviewInput{Approve: true, Reason: "Old evidence accepted"})
		require.Error(t, err)
		_, err = service.CurrentOwner(ctx, users[0], "Claim0005")
		require.Error(t, err)
		cancelled, err := service.Detail(ctx, users[0], pending.Claim.ID, false)
		require.NoError(t, err)
		require.Equal(t, "CANCELLED", cancelled.Status)
		fresh, err := service.Create(ctx, users[1], sitemanagement.SiteClaimCreateInput{ShortID: "Claim0005", Method: sitemanagement.Meta})
		require.NoError(t, err)
		_, err = service.Check(ctx, users[1], fresh.Claim.ID)
		require.NoError(t, err)
		require.Error(t, service.Revoke(ctx, reviewer, "Claim0005", revoke))
		latest, err := service.CurrentOwner(ctx, users[1], "Claim0005")
		require.NoError(t, err)
		require.NotEqual(t, owner.ID, latest.ID)
	})
	t.Run("account reads remain available while a writer holds a site row lock", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()
		_, err = tx.Exec(ctx, `SELECT id FROM directory.sites WHERE short_id='Claim0005' FOR UPDATE`)
		require.NoError(t, err)
		read, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		_, err = service.Context(read, users[1], "Claim0005")
		require.NoError(t, err)
	})
}
