package siteaudit

import (
	"context"
	"heyblog-api/internal/features/auth"
	"net/http"
)

func (service *AccountService) Submissions(ctx context.Context, user auth.User, page int32) (AccountSubmissionPage, error) {
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
	if err == nil && audit.BaseSnapshot.resubmission {
		audit.Action = ActionCreate
		audit.SiteID = ""
		audit.BaseSnapshot = Snapshot{}
		audit.ProposedSnapshot.SiteID = ""
		audit.ProposedSnapshot.ShortID = ""
		audit.ProposedSnapshot.CustomID = ""
		audit.ProposedSnapshot.Components = submittedProgram(audit.ProposedSnapshot.Components)
		audit.CurrentSnapshot = Snapshot{}
		audit.EffectiveSnapshot = Snapshot{}
		audit.ReviewDraftSnapshot = nil
		audit.Diff = DiffViews{}
		audit.HasCurrentSnapshot = false
	}
	return audit, err
}

func submittedProgram(components []ComponentSnapshot) []ComponentSnapshot {
	var result []ComponentSnapshot
	for _, component := range components {
		if component.Role == "SITE_PROGRAM" {
			result = append(result, component)
		}
	}
	return result
}
