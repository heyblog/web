package siteaudit

import (
	"context"
	"strings"

	"heyblog-api/internal/features/auth"
)

func (service *Service) SaveReviewDraft(ctx context.Context, reviewer auth.User, input ReviewDraftInput) (Audit, error) {
	auditID, reviewerID, err := reviewDraftIDs(reviewer, input.AuditID)
	if err != nil {
		return Audit{}, err
	}
	if service.slugGenerator != nil {
		var pending Snapshot
		err = service.repository.InTransaction(ctx, func(transaction AuditTransaction) error {
			_, current, contextErr := loadReviewDraftContext(ctx, transaction, draftVersion{AuditID: auditID, ExpectedSiteRevision: input.ExpectedSiteRevision, ExpectedReviewDraftRevision: input.ExpectedReviewDraftRevision})
			if contextErr != nil {
				return contextErr
			}
			var buildErr error
			pending, buildErr = BuildProposedSnapshot(input.Site, current)
			return buildErr
		})
		if err != nil {
			return Audit{}, err
		}
		generated, generationErr := service.slugGenerator(ctx, reviewer, pending.Tags)
		if generationErr != nil {
			return Audit{}, generationErr
		}
		for i, tag := range input.Site.Tags {
			if strings.TrimSpace(tag.ID) == "" && strings.TrimSpace(tag.Slug) == "" {
				for _, candidate := range generated {
					if candidate.SuggestedName == strings.TrimSpace(tag.SuggestedName) && candidate.Description == strings.TrimSpace(tag.Description) {
						input.Site.Tags[i].Slug = candidate.Slug
						break
					}
				}
			}
		}
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
		draft, buildErr = transaction.PrepareSubmission(ctx, draft, current)
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
