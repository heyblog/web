package siteaudit

import (
	"context"
	"heyblog-api/internal/features/auth"
	"net/http"
)

func (service *AccountService) Submissions(ctx context.Context, user auth.User, page int32) ([]AuditListItem, error) {
	return service.repository.accountSubmissions(ctx, user.ID, page)
}
func (service *AccountService) Submission(ctx context.Context, user auth.User, auditID string) (Audit, error) {
	allowed, err := service.repository.canReadAccountAudit(ctx, user.ID, auditID)
	if err != nil {
		return Audit{}, err
	}
	if !allowed {
		return Audit{}, newServiceError("audit_not_found", http.StatusNotFound, "the account submission was not found")
	}
	audit, err := service.audits.AuditDetail(ctx, auditID)
	// A shared target request does not disclose another submitter's private contact data.
	if audit.SubmitterUserID != user.ID {
		audit.SubmitterName = ""
		audit.SubmitterEmail = ""
		audit.SubmitterUserID = ""
		audit.RequestReason = ""
	}
	return audit, err
}
