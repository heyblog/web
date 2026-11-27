//go:build integration

package integration_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"heyblog-api/internal/domain/proof"
	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/sitemanagement"

	"heyblog.local/taskprotocol"

	"heyblog-api/internal/features/taskcenter"

	"github.com/stretchr/testify/require"
)

func TestHTTPClaimsRequireWorkerAttempts(t *testing.T) {
	for _, method := range []sitemanagement.Method{sitemanagement.Meta, sitemanagement.File} {
		t.Run(string(method), func(t *testing.T) {
			h := newTaskHarness(t, "verification")
			var userID string
			require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `INSERT INTO identity.users(email,username,display_name,role)
				VALUES('claim@example.test','claim_worker','Claim','SYS_ADMIN') RETURNING id::text`).Scan(&userID))
			user := auth.User{ID: userID, Role: auth.RoleSysAdmin}
			_, err := h.fixture.pool.Exec(t.Context(), `UPDATE operations.request_configs SET parameters=jsonb_set(parameters,'{signed}','true') WHERE purpose='verification'`)
			require.NoError(t, err)
			for _, platform := range []proof.Platform{proof.Cloudflare, proof.EdgeOne} {
				key, err := h.signing.Generate(t.Context(), userID, platform)
				require.NoError(t, err)
				for _, action := range []string{"publish", "activate"} {
					require.NoError(t, h.signing.Transition(t.Context(), userID, key, action))
				}
			}
			insertBlogSite(t.Context(), t, h.fixture.pool, "HTTPprf01", "HTTP proof", "proof.example.test")
			store := sitemanagement.NewRepository(h.fixture.pool)
			// A nil synchronous verifier ensures HTTP checks can only use the queue.
			service := sitemanagement.NewService(store, nil, nil)
			service.SetVerificationQueue(h.repo)
			created, err := service.Create(t.Context(), user, sitemanagement.SiteClaimCreateInput{ShortID: "HTTPprf01", Method: method})
			require.NoError(t, err)
			_, err = service.Check(t.Context(), user, created.Claim.ID)
			var failure *sitemanagement.ServiceError
			require.ErrorAs(t, err, &failure)
			require.Equal(t, "verification_async_required", failure.Code)
			queued, err := service.StartCheck(t.Context(), user, created.Claim.ID)
			require.NoError(t, err)
			require.NotNil(t, queued.Task)
			repeated, err := service.StartCheck(t.Context(), user, created.Claim.ID)
			require.NoError(t, err)
			require.Equal(t, queued.Task.ID, repeated.Task.ID)
			claim, err := store.Claim(t.Context(), created.Claim.ID)
			require.NoError(t, err)
			require.Equal(t, "PENDING", claim.Status)
			h.now = time.Now().UTC().Add(time.Second)
			w, err := h.repo.Claim(t.Context(), h.worker)
			require.NoError(t, err)
			require.NotNil(t, w)
			require.Equal(t, queued.Task.ID, w.RunID)
			require.Equal(t, "verification", w.Kind)
			require.True(t, w.SameOrigin)
			body := []byte(fmt.Sprintf(`<html><head><meta name="heyblog-site-verification" content="%s"></head></html>`, created.Token))
			if method == sitemanagement.File {
				require.True(t, strings.HasSuffix(w.Address, "/.well-known/heyblog-site-verification.txt"))
				body = []byte(created.Token)
			}
			// A mismatch settles the check without verifying ownership or trying a fallback.
			mismatch := h.report(t, w, w.Route[0], 0, 200, []byte("not the proof"), nil)
			require.Equal(t, "proof_mismatch", mismatch.Reason)
			require.Len(t, w.Route, 2)
			// Recover one method after receipt but before completion.
			if method == sitemanagement.File {
				h.now = h.now.Add(3 * time.Minute)
				_, err = h.repo.WorkerHeartbeat(t.Context(), h.worker)
				require.NoError(t, err)
				w, err = h.repo.Claim(t.Context(), h.worker)
				require.NoError(t, err)
				require.NotNil(t, w)
				require.True(t, taskprotocol.BusinessComplete(w.Kind, w.Progress[w.Route[0]]))
			}
			result := h.finish(t, w)
			require.False(t, result.BusinessValid)
			require.False(t, result.Fallback)
			require.Len(t, result.Paths, 1)
			require.Equal(t, "proof_mismatch", result.Paths[0].ErrorKind)
			pending, err := store.Claim(t.Context(), created.Claim.ID)
			require.NoError(t, err)
			require.Equal(t, "PENDING", pending.Status)
			finished, err := h.repo.ClaimTask(t.Context(), created.Claim.ID, user.ID)
			require.NoError(t, err)
			require.Equal(t, "succeeded", finished.State)
			require.Equal(t, "proof_mismatch", finished.Reason)
			var requests, received int
			require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `SELECT count(*),count(*) FILTER (WHERE s.received)
				FROM operations.request_attempts a JOIN operations.request_statistics s ON s.attempt_id=a.id WHERE a.target_id=$1`, w.ID).Scan(&requests, &received))
			require.Equal(t, 1, requests)
			require.Equal(t, 1, received)
			// Correcting the published proof creates a new task for the same challenge.
			next, err := service.StartCheck(t.Context(), user, created.Claim.ID)
			require.NoError(t, err)
			require.NotEqual(t, queued.Task.ID, next.Task.ID)
			w, err = h.repo.Claim(t.Context(), h.worker)
			require.NoError(t, err)
			require.NotNil(t, w)
			h.report(t, w, w.Route[0], 0, 200, body, nil)
			require.True(t, h.finish(t, w).BusinessValid)
			verified, err := store.Claim(t.Context(), created.Claim.ID)
			require.NoError(t, err)
			require.Equal(t, "VERIFIED", verified.Status)
			var attempts int
			require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `SELECT count(*) FROM operations.request_attempts WHERE target_id=$1 AND state='received'`, w.ID).Scan(&attempts))
			require.Equal(t, 1, attempts)
		})
	}
}

func TestWorkerEmailChangeProofRejectsStaleResults(t *testing.T) {
	h := newTaskHarness(t, "verification")
	ctx := t.Context()
	var user auth.User
	require.NoError(t, h.fixture.pool.QueryRow(ctx, `INSERT INTO identity.users(email,username,display_name)
        VALUES('scoped-proof@example.test','scoped_proof','Scope') RETURNING id::text`).Scan(&user.ID))
	siteID := insertBlogSite(ctx, t, h.fixture.pool, "Scoped001", "Scoped proof", "scoped.example.test")
	store := sitemanagement.NewRepository(h.fixture.pool)
	service := sitemanagement.NewService(store, nil, claimsProofVerifier{})
	service.SetVerificationQueue(h.repo)
	ordinary, err := service.Create(ctx, user, sitemanagement.SiteClaimCreateInput{ShortID: "Scoped001", Method: sitemanagement.DNS})
	require.NoError(t, err)
	_, err = service.Check(ctx, user, ordinary.Claim.ID)
	require.NoError(t, err)
	_, err = h.fixture.pool.Exec(ctx, `UPDATE operations.request_configs SET parameters=jsonb_set(parameters,'{signed}','true') WHERE purpose='verification'`)
	require.NoError(t, err)
	for _, platform := range []proof.Platform{proof.Cloudflare, proof.EdgeOne} {
		key, err := h.signing.Generate(ctx, user.ID, platform)
		require.NoError(t, err)
		for _, action := range []string{"publish", "activate"} {
			require.NoError(t, h.signing.Transition(ctx, user.ID, key, action))
		}
	}
	for _, change := range []string{"regenerated", "cancelled", "expired", "verified"} {
		t.Run(change, func(t *testing.T) {
			ctx := t.Context()
			var requestID string
			require.NoError(t, h.fixture.pool.QueryRow(ctx, `INSERT INTO identity.email_change_requests(user_id,auth_version,old_email,new_email,expires_at)
                SELECT id,auth_version,email,'new-scoped-proof@example.test',clock_timestamp()+interval '7 days'
                FROM identity.users WHERE id=$1 RETURNING id::text`, user.ID).Scan(&requestID))
			created, err := service.Create(ctx, user, sitemanagement.SiteClaimCreateInput{
				ShortID: "Scoped001", EmailChangeID: requestID, Method: sitemanagement.File,
			})
			require.NoError(t, err)
			_, err = service.StartCheck(ctx, user, created.Claim.ID)
			require.NoError(t, err)
			h.now = time.Now().UTC().Add(time.Second)
			work, err := h.repo.Claim(ctx, h.worker)
			require.NoError(t, err)
			require.NotNil(t, work)
			permit, err := h.signing.Permit(ctx, work.ID, h.authorization(work, work.Route[0], 0))
			require.NoError(t, err)
			require.NoError(t, h.repo.Sent(ctx, work.ID, taskcenter.Send{Fence: h.fence(work), AttemptID: permit.ID}))
			switch change {
			case "regenerated":
				_, err = service.Regenerate(ctx, user, created.Claim.ID)
			case "cancelled":
				_, err = h.fixture.pool.Exec(ctx, `DELETE FROM identity.email_change_requests WHERE id=$1`, requestID)
				require.NoError(t, err)
				err = store.CloseInactiveEmailChangeClaims(ctx)
			case "expired":
				_, err = h.fixture.pool.Exec(ctx, `UPDATE identity.email_change_requests
                    SET created_at=clock_timestamp()-interval '8 days', expires_at=clock_timestamp()-interval '1 day'
                    WHERE id=$1`, requestID)
			}
			require.NoError(t, err)
			result, err := h.execution.Report(ctx, work.ID, taskprotocol.Report{
				Fence: h.fence(work), AttemptID: permit.ID, Status: 200, Body: []byte(created.Token),
			})
			if change == "verified" {
				require.NoError(t, err)
				require.True(t, result.Valid)
				require.True(t, h.finish(t, work).BusinessValid)
			} else {
				require.Error(t, err)
				reclaimed, err := h.repo.Claim(ctx, h.worker)
				require.NoError(t, err)
				require.Nil(t, reclaimed)
			}
			claim, err := store.Claim(ctx, created.Claim.ID)
			require.NoError(t, err)
			if change == "verified" {
				require.Equal(t, "VERIFIED", claim.Status)
				require.NotNil(t, claim.ConsumedAt)
			} else {
				require.NotEqual(t, "VERIFIED", claim.Status)
			}
			var installed string
			var revision int64
			require.NoError(t, h.fixture.pool.QueryRow(ctx, `SELECT verified_claim_id::text,revision
                FROM directory.site_ownerships WHERE site_id=$1`, siteID).Scan(&installed, &revision))
			require.Equal(t, ordinary.Claim.ID, installed)
			require.EqualValues(t, 1, revision)
			_, err = h.fixture.pool.Exec(ctx, `DELETE FROM identity.email_change_requests WHERE id=$1`, requestID)
			require.NoError(t, err)
			require.NoError(t, store.CloseInactiveEmailChangeClaims(ctx))
		})
	}
}
