package siteaudit

import (
	"context"
	"fmt"

	"heyblog-api/internal/infrastructure/mail"
)

func (service *Service) notifyDecision(ctx context.Context, audit Audit) {
	if !audit.NotifyByEmail || audit.SubmitterEmail == "" || service.mailer == nil {
		return
	}
	err := service.mailer.SendDecision(ctx, mail.SubmissionDecision{Recipient: audit.SubmitterEmail, Action: string(audit.Action), Status: string(audit.Status), ReviewerComment: audit.ReviewerComment})
	if err != nil && service.logger != nil {
		service.logger.WarnContext(ctx, "submission decision email failed", "event", "submission_decision_email_failed", "audit_id", audit.ID, "error_type", fmt.Sprintf("%T", err))
	}
}
