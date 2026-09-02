package siteaudit

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"heyblog-api/internal/auth"
	dbgen "heyblog-api/internal/database/gen"
	"heyblog-api/internal/mail"
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
	reviewerID, err := parseUUID(reviewer.ID)
	if err != nil {
		return Audit{}, fmt.Errorf("parse reviewer ID: %w", err)
	}
	auditID, err := parseUUID(input.AuditID)
	if err != nil {
		return Audit{}, newServiceError("audit_not_found", http.StatusNotFound, "the audit was not found")
	}
	var reviewed Audit
	err = service.repository.InTransaction(ctx, func(queries *dbgen.Queries) error {
		row, lockErr := queries.LockSiteAuditByID(ctx, auditID)
		if lockErr != nil {
			if errors.Is(lockErr, pgx.ErrNoRows) {
				return newServiceError("audit_not_found", http.StatusNotFound, "the audit was not found")
			}
			return fmt.Errorf("lock site audit: %w", lockErr)
		}
		if row.Status != string(StatusPending) {
			return newServiceError("audit_already_reviewed", http.StatusConflict, "the audit has already been reviewed")
		}
		if input.ExpectedReviewDraftRevision != row.ReviewDraftRevision {
			return newServiceError("review_draft_changed", http.StatusConflict, "the reviewer correction changed; refresh before deciding")
		}
		if input.Decision == DecisionReject {
			updated, rejectErr := queries.RejectSiteAudit(ctx, dbgen.RejectSiteAuditParams{ReviewerComment: stringPointer(input.ReviewerComment), ReviewedBy: reviewerID, ID: auditID})
			if rejectErr != nil {
				return fmt.Errorf("reject site audit: %w", rejectErr)
			}
			reviewed, rejectErr = auditFromRow(updated)
			return rejectErr
		}
		return service.approve(ctx, queries, reviewer, row, input, reviewerID, &reviewed)
	})
	if err != nil {
		return Audit{}, err
	}
	service.notifyDecision(ctx, reviewed)
	return reviewed, nil
}

func (service *Service) notifyDecision(ctx context.Context, audit Audit) {
	if !audit.NotifyByEmail || audit.SubmitterEmail == "" || service.mailer == nil {
		return
	}
	err := service.mailer.SendDecision(ctx, mail.SubmissionDecision{Recipient: audit.SubmitterEmail, Action: string(audit.Action), Status: string(audit.Status), ReviewerComment: audit.ReviewerComment})
	if err != nil && service.logger != nil {
		service.logger.WarnContext(ctx, "submission decision email failed", "event", "submission_decision_email_failed", "audit_id", audit.ID, "error_type", fmt.Sprintf("%T", err))
	}
}

func (service *Service) approve(
	ctx context.Context,
	queries *dbgen.Queries,
	reviewer auth.User,
	row dbgen.DirectorySiteAudit,
	input ReviewInput,
	reviewerID pgtype.UUID,
	destination *Audit,
) error {
	audit, err := auditFromRow(row)
	if err != nil {
		return err
	}
	current := audit.BaseSnapshot
	if audit.Action != ActionCreate {
		if _, err := queries.LockSiteByID(ctx, row.SiteID); err != nil {
			return fmt.Errorf("lock target site: %w", err)
		}
		current, err = loadSnapshot(ctx, queries, row.SiteID)
		if err != nil {
			return err
		}
		if input.ExpectedSiteRevision != current.Revision {
			return newServiceError("site_revision_changed", http.StatusConflict, "the site changed; refresh the three-way diff before reviewing")
		}
	}
	final, conflicts := MergeRequestedSnapshot(audit.BaseSnapshot, audit.ProposedSnapshot, current)
	if audit.ReviewDraftSnapshot != nil {
		final = *audit.ReviewDraftSnapshot
	}
	if len(conflicts) > 0 && audit.ReviewDraftSnapshot == nil {
		return newServiceError("audit_conflicts_unresolved", http.StatusConflict, "the three-way diff contains unresolved conflicts")
	}
	createsProgramDependencies := false
	for _, component := range final.Components {
		if component.Role == "SITE_PROGRAM" && component.ID == "" {
			createsProgramDependencies = true
			break
		}
	}
	final, err = resolveTaxonomy(ctx, queries, reviewer, final)
	if err != nil {
		return err
	}
	siteID, appliedRevision, err := service.applySnapshot(ctx, queries, audit.Action, current, final, row.SiteID, reviewerID, createsProgramDependencies)
	if err != nil {
		return err
	}
	final.SiteID, _ = uuidString(siteID)
	final.Revision = appliedRevision
	finalJSON, err := encodeSnapshot(final)
	if err != nil {
		return err
	}
	updated, err := queries.ApproveSiteAudit(ctx, dbgen.ApproveSiteAuditParams{SiteID: siteID, FinalSnapshot: finalJSON, ReviewerComment: stringPointer(input.ReviewerComment), ReviewedBy: reviewerID, ID: row.ID})
	if err != nil {
		return fmt.Errorf("approve site audit: %w", err)
	}
	*destination, err = auditFromRow(updated)
	return err
}
