//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/sitemanagement"
)

func verifyEmailChangeSiteProofs(ctx context.Context, t *testing.T, pool *pgxpool.Pool, reviewer auth.User) {
	t.Helper()
	var user auth.User
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO identity.users(email,username,display_name)
		VALUES('site-proof@example.test','site_proof','Site Proof') RETURNING id::text`).Scan(&user.ID))
	repository := sitemanagement.NewRepository(pool)
	service := sitemanagement.NewService(repository, nil, claimsProofVerifier{})
	newRequest := func() string {
		var requestID string
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO identity.email_change_requests(user_id,auth_version,old_email,new_email,expires_at)
			SELECT id,auth_version,email,'new-site-proof@example.test',clock_timestamp()+interval '7 days'
			FROM identity.users WHERE id=$1::uuid RETURNING id::text`, user.ID).Scan(&requestID))
		return requestID
	}
	create := func(shortID, requestID string, method sitemanagement.Method) sitemanagement.Claim {
		result, err := service.Create(ctx, user, sitemanagement.SiteClaimCreateInput{
			ShortID: shortID, EmailChangeID: requestID, Method: method,
			Evidence:    "The author placed an independent verification statement on the site",
			EvidenceURL: "https://proof-one.example.test/verification",
		})
		require.NoError(t, err)
		return result.Claim
	}
	approve := func(claim sitemanagement.Claim) {
		_, err := service.Check(ctx, user, claim.ID)
		require.NoError(t, err)
	}
	complete := func(requestID string, expected bool) {
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()
		var applied bool
		require.NoError(t, tx.QueryRow(ctx, `SELECT true FROM identity.users WHERE id=$1::uuid FOR UPDATE`, user.ID).Scan(&applied))
		require.NoError(t, tx.QueryRow(ctx, `SELECT directory.complete_email_change_site_proofs($1::uuid,$2::uuid)`, user.ID, requestID).Scan(&applied))
		require.Equal(t, expected, applied)
		if applied {
			_, err = tx.Exec(ctx, `DELETE FROM identity.email_change_requests WHERE id=$1::uuid`, requestID)
			require.NoError(t, err)
		}
		require.NoError(t, tx.Commit(ctx))
	}

	const first, second = "EmProof01", "EmProof02"
	siteID := insertBlogSite(ctx, t, pool, first, "First proof site", "proof-one.example.test")
	ordinary := create(first, "", sitemanagement.File)
	approve(ordinary)
	requestID := newRequest()
	complete(requestID, false)
	oldProof := create(first, requestID, sitemanagement.DNS)
	approve(oldProof)
	current, err := service.CurrentOwner(ctx, user, first)
	require.NoError(t, err)
	require.EqualValues(t, 1, current.Revision)
	proof, err := repository.Claim(ctx, oldProof.ID)
	require.NoError(t, err)
	require.NotNil(t, proof.ConsumedAt)

	// A site returning to the blog directory needs a new proof.
	for _, status := range []string{"NON_BLOG", "BLOG"} {
		_, err := pool.Exec(ctx, `UPDATE directory.sites SET blog_status=$1 WHERE id=$2`, status, siteID)
		require.NoError(t, err)
	}
	complete(requestID, false)
	approve(create(first, requestID, sitemanagement.DNS))

	// A site move invalidates the scoped proof even if the address returns.
	for _, host := range []string{"proof-moved.example.test", "proof-one.example.test"} {
		_, err := pool.Exec(ctx, `UPDATE directory.sites SET normalized_host=$1 WHERE id=$2`, host, siteID)
		require.NoError(t, err)
	}
	complete(requestID, false)
	approve(create(first, requestID, sitemanagement.Meta))
	secondID := insertBlogSite(ctx, t, pool, second, "Second proof site", "proof-two.example.test")
	approve(create(second, "", sitemanagement.File))
	complete(requestID, false)
	manual := create(first, requestID, sitemanagement.Manual)
	approved, err := service.Review(ctx, reviewer, manual.ID, sitemanagement.SiteClaimReviewInput{
		Approve: true, Reason: "Independent evidence reviewed",
	})
	require.NoError(t, err)
	require.Equal(t, "VERIFIED", approved.Status)
	_, err = pool.Exec(ctx, `UPDATE directory.sites SET visibility='REMOVED' WHERE id=$1`, secondID)
	require.NoError(t, err)
	complete(requestID, true)

	var installed string
	require.NoError(t, pool.QueryRow(ctx, `SELECT verified_claim_id::text FROM directory.site_ownerships WHERE site_id=$1`, siteID).Scan(&installed))
	require.Equal(t, manual.ID, installed)
	requestID = newRequest()
	complete(requestID, false)
	expired := create(first, requestID, sitemanagement.File)
	_, err = pool.Exec(ctx, `UPDATE identity.email_change_requests
		SET created_at=clock_timestamp()-interval '8 days',expires_at=clock_timestamp()-interval '1 day'
		WHERE id=$1::uuid`, requestID)
	require.NoError(t, err)
	_, err = service.Check(ctx, user, expired.ID)
	require.ErrorContains(t, err, "expired")
	require.NoError(t, repository.CloseInactiveEmailChangeClaims(ctx))
	closed, err := repository.Claim(ctx, expired.ID)
	require.NoError(t, err)
	require.Equal(t, "CANCELLED", closed.Status)
	complete(requestID, false)
}
