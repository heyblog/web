package siteaudit

import (
	"context"
	"fmt"
	"net/http"

	"heyblog-api/internal/features/auth"
)

func reviewDraftIDs(reviewer auth.User, rawAuditID string) (string, string, error) {
	if !canReview(reviewer) {
		return "", "", newServiceError("forbidden", http.StatusForbidden, "site audit review permission is required")
	}
	if _, err := parseUUID(reviewer.ID); err != nil {
		return "", "", fmt.Errorf("parse reviewer ID: %w", err)
	}
	if _, err := parseUUID(rawAuditID); err != nil {
		return "", "", newServiceError("audit_not_found", http.StatusNotFound, "the audit was not found")
	}
	return rawAuditID, reviewer.ID, nil
}
func loadReviewDraftContext(ctx context.Context, transaction AuditTransaction, version draftVersion) (Audit, Snapshot, error) {
	row, err := transaction.LockAudit(ctx, version.AuditID)
	if err != nil {
		return Audit{}, Snapshot{}, err
	}
	if row.Status != StatusPending {
		return Audit{}, Snapshot{}, newServiceError("audit_already_reviewed", http.StatusConflict, "the audit has already been reviewed")
	}
	if row.Action != ActionCreate && row.Action != ActionUpdate {
		return Audit{}, Snapshot{}, newServiceError("review_draft_forbidden", http.StatusUnprocessableEntity, "only create and update audits allow reviewer corrections")
	}
	if row.ReviewDraftRevision != version.ExpectedReviewDraftRevision {
		return Audit{}, Snapshot{}, newServiceError("review_draft_changed", http.StatusConflict, "the reviewer correction changed; refresh before continuing")
	}
	audit, err := transaction.ReadLockedAudit()
	if err != nil {
		return Audit{}, Snapshot{}, err
	}
	current := audit.ProposedSnapshot
	if row.Action == ActionUpdate {
		current, err = transaction.LockSiteSnapshot(ctx, row.SiteID)
		if err != nil {
			return Audit{}, Snapshot{}, err
		}
	}
	if current.Revision != version.ExpectedSiteRevision {
		return Audit{}, Snapshot{}, newServiceError("site_revision_changed", http.StatusConflict, "the site changed; refresh before continuing")
	}
	return row, current, nil
}
