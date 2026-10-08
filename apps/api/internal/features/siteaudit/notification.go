package siteaudit

import (
	"context"
	"fmt"

	"heyblog-api/internal/infrastructure/mail"
)

func (service *Service) notifyDecision(ctx context.Context, audit Audit) {
	if service.mailer == nil {
		return
	}
	recipients := map[string]bool{}
	if audit.NotifyByEmail && audit.SubmitterEmail != "" {
		recipients[audit.SubmitterEmail] = audit.SourceChannel != "ANONYMOUS"
	}
	owners, err := service.repository.DecisionRecipients(ctx, audit.ID)
	if err != nil && service.logger != nil {
		service.logger.WarnContext(ctx, "friend decision recipients unavailable", "event", "friend_decision_recipients_failed", "audit_id", audit.ID, "error_type", fmt.Sprintf("%T", err))
	}
	for _, recipient := range owners {
		recipients[recipient] = true
	}
	for recipient, account := range recipients {
		err := service.mailer.SendDecision(ctx, mail.SubmissionDecision{Recipient: recipient, Action: string(audit.Action), Status: string(audit.Status), ReviewerComment: audit.ReviewerComment, Account: account})
		if err != nil && service.logger != nil {
			service.logger.WarnContext(ctx, "submission decision email failed", "event", "submission_decision_email_failed", "audit_id", audit.ID, "error_type", fmt.Sprintf("%T", err))
		}
	}
}
