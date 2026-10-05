package siteaudit

import (
	"context"
	"net/http"

	"heyblog-api/internal/features/auth"
)

// Preflight checks the same audit/site revisions, then releases locks before AI I/O.
// The final approval repeats these checks and canonical resolution inside its transaction.
func (service *Service) prepareReviewSlugs(ctx context.Context, reviewer auth.User, input ReviewInput) ([]TagSnapshot, error) {
	var tags []TagSnapshot
	err := service.repository.InTransaction(ctx, func(transaction AuditTransaction) error {
		row, err := transaction.LockAudit(ctx, input.AuditID)
		if err != nil {
			return err
		}
		if row.Status != StatusPending {
			return newServiceError("audit_already_reviewed", http.StatusConflict, "the audit has already been reviewed")
		}
		if row.ReviewDraftRevision != input.ExpectedReviewDraftRevision {
			return newServiceError("review_draft_changed", http.StatusConflict, "the reviewer correction changed; refresh before deciding")
		}
		audit, err := transaction.ReadLockedAudit(ctx)
		if err != nil {
			return err
		}
		current := audit.BaseSnapshot
		if audit.Action != ActionCreate {
			current, err = transaction.LockSiteSnapshot(ctx, audit.SiteID)
			if err != nil {
				return err
			}
			if current.Revision != input.ExpectedSiteRevision {
				return newServiceError("site_revision_changed", http.StatusConflict, "the site changed; refresh before reviewing")
			}
		}
		final, conflicts := MergeRequestedSnapshot(audit.BaseSnapshot, audit.ProposedSnapshot, current)
		if audit.ReviewDraftSnapshot != nil {
			final = *audit.ReviewDraftSnapshot
		} else if len(conflicts) > 0 {
			return newServiceError("audit_conflicts_unresolved", http.StatusConflict, "resolve the audit conflicts before approving")
		}
		tags = final.Tags
		return nil
	})
	if err != nil {
		return nil, err
	}
	return service.slugGenerator(ctx, reviewer, tags)
}
