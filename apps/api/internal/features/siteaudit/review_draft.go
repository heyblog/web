package siteaudit

import (
	"context"

	"heyblog-api/internal/features/auth"
)

func (service *Service) SaveReviewDraft(ctx context.Context, reviewer auth.User, input ReviewDraftInput) (Audit, error) {
	auditID, reviewerID, err := reviewDraftIDs(reviewer, input.AuditID)
	if err != nil {
		return Audit{}, err
	}
	var saved Audit
	err = service.repository.InTransaction(ctx, func(transaction AuditTransaction) error {
		row, current, contextErr := loadReviewDraftContext(ctx, transaction, draftVersion{AuditID: auditID, ExpectedSiteRevision: input.ExpectedSiteRevision, ExpectedReviewDraftRevision: input.ExpectedReviewDraftRevision})
		if contextErr != nil {
			return contextErr
		}
		draft, buildErr := BuildProposedSnapshot(input.Site, current)
		if buildErr != nil {
			return buildErr
		}
		draft, buildErr = service.repository.PrepareSubmission(ctx, draft)
		if buildErr != nil {
			return buildErr
		}
		var updateErr error
		saved, updateErr = transaction.SaveDraft(ctx, draftRecord{AuditID: row.ID, ReviewerID: reviewerID, ExpectedRevision: input.ExpectedReviewDraftRevision, Snapshot: draft})
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
	err = service.repository.InTransaction(ctx, func(transaction AuditTransaction) error {
		row, _, contextErr := loadReviewDraftContext(ctx, transaction, draftVersion{AuditID: auditID, ExpectedSiteRevision: input.ExpectedSiteRevision, ExpectedReviewDraftRevision: input.ExpectedReviewDraftRevision})
		if contextErr != nil {
			return contextErr
		}
		var updateErr error
		discarded, updateErr = transaction.DiscardDraft(ctx, draftRecord{AuditID: row.ID, ReviewerID: reviewerID, ExpectedRevision: input.ExpectedReviewDraftRevision})
		return updateErr
	})
	return discarded, err
}
