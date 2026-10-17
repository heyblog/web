//go:build integration

package integration_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/sitemanagement"

	"github.com/stretchr/testify/require"
)

func TestManualClaimEvidenceLimitsAndLifecycle(t *testing.T) {
	fixture := newAuditMigrationFixture(t)
	_, err := fixture.provider.Up(t.Context())
	require.NoError(t, err)
	ctx := t.Context()
	repo := sitemanagement.NewRepository(fixture.pool)
	service := sitemanagement.NewService(repo, nil, claimsProofVerifier{})
	user := auth.User{Role: auth.RoleUser}
	require.NoError(t, fixture.pool.QueryRow(ctx, `INSERT INTO identity.users(email,username,display_name) VALUES ('manual@example.test','manual_user','Manual') RETURNING id::text`).Scan(&user.ID))
	siteID := insertSite(ctx, t, fixture.pool, "Manual001", "Manual evidence", "manual-evidence.example.com")
	target, err := repo.Site(ctx, "Manual001")
	require.NoError(t, err)
	ready := func(name string, size int64) sitemanagement.ClaimAttachment {
		attachment, err := repo.CreateAttachmentMetadata(ctx, user.ID, target, sitemanagement.AttachmentUploadInput{Filename: name, ContentType: "application/pdf", Size: size, StorageIdentity: "https://source.cos.region.myqcloud.com"})
		require.NoError(t, err)
		result, err := repo.CompleteAttachmentMetadata(ctx, user.ID, attachment.ID, func(row sitemanagement.ClaimAttachment) (sitemanagement.EvidenceObject, error) {
			return sitemanagement.EvidenceObject{Key: row.StorageKey, Size: row.Size, ContentType: row.ContentType, SHA256: strings.Repeat("a", 64)}, nil
		})
		require.NoError(t, err)
		return result
	}
	files := []sitemanagement.ClaimAttachment{ready("first.pdf", 10<<20), ready("second.pdf", 10<<20), ready("third.pdf", 1)}
	input := sitemanagement.SiteClaimCreateInput{ShortID: "Manual001", Method: sitemanagement.Manual, Evidence: "Public ownership statement and verifiable proof", AttachmentIDs: []string{files[0].ID, files[1].ID, files[2].ID}}
	_, err = service.Create(ctx, user, input)
	require.ErrorContains(t, err, "20 MiB")
	input.AttachmentIDs = input.AttachmentIDs[:2]
	created, err := service.Create(ctx, user, input)
	require.NoError(t, err)
	require.Len(t, created.Claim.Attachments, 2)
	// Duplicate creates never mutate the original evidence or add more files.
	input.AttachmentIDs = []string{files[2].ID}
	repeated, err := service.Create(ctx, user, input)
	require.NoError(t, err)
	require.Equal(t, created.Claim.ID, repeated.Claim.ID)
	require.Len(t, repeated.Claim.Attachments, 2)
	require.ErrorContains(t, repo.DeleteAttachmentMetadata(ctx, user.ID, files[0].ID), "submitted claim")
	// Repeated deletion of an unused upload is idempotent.
	require.NoError(t, repo.DeleteAttachmentMetadata(ctx, user.ID, files[2].ID))
	require.NoError(t, repo.DeleteAttachmentMetadata(ctx, user.ID, files[2].ID))
	_, err = service.Cancel(ctx, user, created.Claim.ID)
	require.NoError(t, err)
	var status string
	require.NoError(t, fixture.pool.QueryRow(ctx, `SELECT status FROM directory.site_claim_attachments WHERE id=$1::uuid`, files[0].ID).Scan(&status))
	require.Equal(t, "CLEANUP_PENDING", status)
	// Abandoned uploads expire before cleanup; ready evidence has a full day.
	abandoned, err := repo.CreateAttachmentMetadata(ctx, user.ID, target, sitemanagement.AttachmentUploadInput{Filename: "abandoned.pdf", ContentType: "application/pdf", Size: 10, StorageIdentity: "source"})
	require.NoError(t, err)
	_, err = fixture.admin.Exec(ctx, `UPDATE directory.site_claim_attachments SET created_at=clock_timestamp()-interval '16 minutes' WHERE id=$1::uuid`, abandoned.ID)
	require.NoError(t, err)
	items, err := repo.ListAttachmentCleanup(ctx)
	require.NoError(t, err)
	found := false
	for _, item := range items {
		if item.ID == abandoned.ID {
			found = true
			require.Equal(t, sitemanagement.AttachmentCleanupPending, item.Status)
		}
	}
	require.True(t, found)
	// Removing a site schedules attached evidence without physical deletion.
	removal := ready("remove.pdf", 10)
	_, err = fixture.admin.Exec(ctx, `UPDATE directory.sites SET visibility='REMOVED' WHERE id=$1`, siteID)
	require.NoError(t, err)
	require.NoError(t, fixture.pool.QueryRow(ctx, `SELECT status FROM directory.site_claim_attachments WHERE id=$1::uuid`, removal.ID).Scan(&status))
	require.Equal(t, "CLEANUP_PENDING", status)
}

func TestManualClaimConcurrentAccountQuota(t *testing.T) {
	fixture := newAuditMigrationFixture(t)
	_, err := fixture.provider.Up(t.Context())
	require.NoError(t, err)
	user := auth.User{Role: auth.RoleUser}
	require.NoError(t, fixture.pool.QueryRow(t.Context(), `INSERT INTO identity.users(email,username,display_name) VALUES ('quota@example.test','quota_user','Quota') RETURNING id::text`).Scan(&user.ID))
	service := sitemanagement.NewService(sitemanagement.NewRepository(fixture.pool), nil, claimsProofVerifier{})
	for i := 0; i < 6; i++ {
		insertSite(t.Context(), t, fixture.pool, fmt.Sprintf("Quota%04d", i), "Quota", fmt.Sprintf("quota%d.example.com", i))
	}
	results := make(chan error, 6)
	var group sync.WaitGroup
	for i := 0; i < 6; i++ {
		group.Go(func() {
			_, err := service.Create(t.Context(), user, sitemanagement.SiteClaimCreateInput{ShortID: fmt.Sprintf("Quota%04d", i), Method: sitemanagement.Manual, Evidence: "Public ownership statement and verifiable proof", EvidenceURL: "https://example.com/about"})
			results <- err
		})
	}
	group.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else {
			require.ErrorContains(t, err, "limit")
		}
	}
	require.Equal(t, 3, successes)
}
