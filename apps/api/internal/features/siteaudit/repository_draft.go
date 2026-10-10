package siteaudit

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func reviewRecordIDs(audit, reviewer string) (pgtype.UUID, pgtype.UUID, error) {
	auditID, err := parseUUID(audit)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, err
	}
	reviewerID, err := parseUUID(reviewer)
	return auditID, reviewerID, err
}

func (transaction *auditTransaction) SaveDraft(ctx context.Context, record draftRecord) (Audit, error) {
	encoded, err := encodeSnapshot(record.Snapshot)
	if err != nil {
		return Audit{}, err
	}
	auditID, reviewerID, err := reviewRecordIDs(record.AuditID, record.ReviewerID)
	if err != nil {
		return Audit{}, err
	}
	updated, err := transaction.queries.SaveSiteAuditReviewDraft(ctx, dbgen.SaveSiteAuditReviewDraftParams{
		ReviewDraftSnapshot: encoded, ReviewDraftUpdatedBy: reviewerID, ID: auditID, ExpectedReviewDraftRevision: record.ExpectedRevision,
	})
	if err != nil {
		return Audit{}, mapReviewDraftUpdateError(err, "saving")
	}
	audit, err := auditFromRow(updated)
	if err != nil {
		return Audit{}, err
	}
	return projectAuditTaxonomy(ctx, transaction.queries, audit, true)
}

func (transaction *auditTransaction) DiscardDraft(ctx context.Context, record draftRecord) (Audit, error) {
	auditID, reviewerID, err := reviewRecordIDs(record.AuditID, record.ReviewerID)
	if err != nil {
		return Audit{}, err
	}
	updated, err := transaction.queries.DiscardSiteAuditReviewDraft(ctx, dbgen.DiscardSiteAuditReviewDraftParams{
		ReviewDraftUpdatedBy: reviewerID, ID: auditID, ExpectedReviewDraftRevision: record.ExpectedRevision,
	})
	if err != nil {
		return Audit{}, mapReviewDraftUpdateError(err, "discarding")
	}
	audit, err := auditFromRow(updated)
	if err != nil {
		return Audit{}, err
	}
	return projectAuditTaxonomy(ctx, transaction.queries, audit, true)
}

func mapReviewDraftUpdateError(err error, operation string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return newServiceError("review_draft_changed", 409, "the reviewer correction changed; refresh before continuing")
	}
	return fmt.Errorf("%s site audit review draft: %w", operation, err)
}
