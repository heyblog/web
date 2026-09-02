package siteaudit

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"heyblog-api/internal/auth"
	dbgen "heyblog-api/internal/database/gen"
)

func (service *Service) SaveReviewDraft(ctx context.Context, reviewer auth.User, input ReviewDraftInput) (Audit, error) {
	auditID, reviewerID, err := reviewDraftIDs(reviewer, input.AuditID)
	if err != nil {
		return Audit{}, err
	}
	var saved Audit
	err = service.repository.InTransaction(ctx, func(queries *dbgen.Queries) error {
		row, current, contextErr := loadReviewDraftContext(ctx, queries, auditID, input.ExpectedSiteRevision, input.ExpectedReviewDraftRevision)
		if contextErr != nil {
			return contextErr
		}
		draft, buildErr := BuildProposedSnapshot(input.Site, current)
		if buildErr != nil {
			return buildErr
		}
		draft, buildErr = service.prepareSubmissionTaxonomy(ctx, draft)
		if buildErr != nil {
			return buildErr
		}
		encoded, encodeErr := encodeSnapshot(draft)
		if encodeErr != nil {
			return encodeErr
		}
		updated, updateErr := queries.SaveSiteAuditReviewDraft(ctx, dbgen.SaveSiteAuditReviewDraftParams{
			ReviewDraftSnapshot: encoded, ReviewDraftUpdatedBy: reviewerID,
			ID: row.ID, ExpectedReviewDraftRevision: input.ExpectedReviewDraftRevision,
		})
		if updateErr != nil {
			return mapReviewDraftUpdateError(updateErr, "saving")
		}
		saved, updateErr = auditFromRow(updated)
		return updateErr
	})
	return saved, err
}

func (service *Service) DiscardReviewDraft(ctx context.Context, reviewer auth.User, input DiscardReviewDraftInput) (Audit, error) {
	auditID, reviewerID, err := reviewDraftIDs(reviewer, input.AuditID)
	if err != nil {
		return Audit{}, err
	}
	var discarded Audit
	err = service.repository.InTransaction(ctx, func(queries *dbgen.Queries) error {
		row, _, contextErr := loadReviewDraftContext(ctx, queries, auditID, input.ExpectedSiteRevision, input.ExpectedReviewDraftRevision)
		if contextErr != nil {
			return contextErr
		}
		updated, updateErr := queries.DiscardSiteAuditReviewDraft(ctx, dbgen.DiscardSiteAuditReviewDraftParams{
			ReviewDraftUpdatedBy: reviewerID, ID: row.ID,
			ExpectedReviewDraftRevision: input.ExpectedReviewDraftRevision,
		})
		if updateErr != nil {
			return mapReviewDraftUpdateError(updateErr, "discarding")
		}
		discarded, updateErr = auditFromRow(updated)
		return updateErr
	})
	return discarded, err
}

func reviewDraftIDs(reviewer auth.User, rawAuditID string) (pgtype.UUID, pgtype.UUID, error) {
	if !canReview(reviewer) {
		return pgtype.UUID{}, pgtype.UUID{}, newServiceError("forbidden", http.StatusForbidden, "site audit review permission is required")
	}
	reviewerID, err := parseUUID(reviewer.ID)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("parse reviewer ID: %w", err)
	}
	auditID, err := parseUUID(rawAuditID)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, newServiceError("audit_not_found", http.StatusNotFound, "the audit was not found")
	}
	return auditID, reviewerID, nil
}

func loadReviewDraftContext(ctx context.Context, queries *dbgen.Queries, auditID pgtype.UUID, expectedSiteRevision, expectedDraftRevision int64) (dbgen.DirectorySiteAudit, Snapshot, error) {
	row, err := queries.LockSiteAuditByID(ctx, auditID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dbgen.DirectorySiteAudit{}, Snapshot{}, newServiceError("audit_not_found", http.StatusNotFound, "the audit was not found")
		}
		return dbgen.DirectorySiteAudit{}, Snapshot{}, fmt.Errorf("lock site audit: %w", err)
	}
	if row.Status != string(StatusPending) {
		return dbgen.DirectorySiteAudit{}, Snapshot{}, newServiceError("audit_already_reviewed", http.StatusConflict, "the audit has already been reviewed")
	}
	if row.Action != string(ActionCreate) && row.Action != string(ActionUpdate) {
		return dbgen.DirectorySiteAudit{}, Snapshot{}, newServiceError("review_draft_forbidden", http.StatusUnprocessableEntity, "only create and update audits allow reviewer corrections")
	}
	if row.ReviewDraftRevision != expectedDraftRevision {
		return dbgen.DirectorySiteAudit{}, Snapshot{}, newServiceError("review_draft_changed", http.StatusConflict, "the reviewer correction changed; refresh before continuing")
	}
	audit, err := auditFromRow(row)
	if err != nil {
		return dbgen.DirectorySiteAudit{}, Snapshot{}, err
	}
	current := audit.ProposedSnapshot
	if row.Action == string(ActionUpdate) {
		if _, err := queries.LockSiteByID(ctx, row.SiteID); err != nil {
			return dbgen.DirectorySiteAudit{}, Snapshot{}, fmt.Errorf("lock target site: %w", err)
		}
		current, err = loadSnapshot(ctx, queries, row.SiteID)
		if err != nil {
			return dbgen.DirectorySiteAudit{}, Snapshot{}, err
		}
	}
	if current.Revision != expectedSiteRevision {
		return dbgen.DirectorySiteAudit{}, Snapshot{}, newServiceError("site_revision_changed", http.StatusConflict, "the site changed; refresh before continuing")
	}
	return row, current, nil
}

func mapReviewDraftUpdateError(err error, operation string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return newServiceError("review_draft_changed", http.StatusConflict, "the reviewer correction changed; refresh before continuing")
	}
	return fmt.Errorf("%s site audit review draft: %w", operation, err)
}
