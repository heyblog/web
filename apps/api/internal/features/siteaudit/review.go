package siteaudit

import (
	"context"
	"net/http"
	"strings"

	"heyblog-api/internal/features/auth"
)

func (service *Service) Review(ctx context.Context, reviewer auth.User, input ReviewInput) (Audit, error) {
	if !canReview(reviewer) {
		return Audit{}, newServiceError("forbidden", http.StatusForbidden, "site audit review permission is required")
	}
	input.ReviewerComment = strings.TrimSpace(input.ReviewerComment)
	if input.Decision == DecisionReject && input.ReviewerComment == "" {
		return Audit{}, newServiceError("review_comment_required", http.StatusUnprocessableEntity, "a reviewer comment is required when rejecting an audit")
	}
	if input.Decision != DecisionApprove && input.Decision != DecisionReject {
		return Audit{}, newServiceError("invalid_decision", http.StatusUnprocessableEntity, "the review decision is invalid")
	}
	auditID, reviewerID, err := reviewDraftIDs(reviewer, input.AuditID)
	if err != nil {
		return Audit{}, err
	}
	var generated []TagSnapshot
	if input.Decision == DecisionApprove && service.slugGenerator != nil {
		generated, err = service.prepareReviewSlugs(ctx, reviewer, input)
		if err != nil {
			return Audit{}, err
		}
	}
	var reviewed Audit
	err = service.repository.InTransaction(ctx, func(transaction AuditTransaction) error {
		row, lockErr := transaction.LockAudit(ctx, auditID)
		if lockErr != nil {
			return lockErr
		}
		if row.Status != StatusPending {
			return newServiceError("audit_already_reviewed", http.StatusConflict, "the audit has already been reviewed")
		}
		if input.ExpectedReviewDraftRevision != row.ReviewDraftRevision {
			return newServiceError("review_draft_changed", http.StatusConflict, "the reviewer correction changed; refresh before deciding")
		}
		if input.Decision == DecisionReject {
			var rejectErr error
			reviewed, rejectErr = transaction.Reject(ctx, decisionRecord{AuditID: auditID, ReviewerID: reviewerID, ReviewerComment: input.ReviewerComment})
			return rejectErr
		}
		var approveErr error
		reviewed, approveErr = service.approve(ctx, transaction, reviewContext{Reviewer: reviewer, Input: input, GeneratedTags: generated})
		return approveErr
	})
	if err != nil {
		return Audit{}, err
	}
	service.notifyDecision(ctx, reviewed)
	return reviewed, nil
}
