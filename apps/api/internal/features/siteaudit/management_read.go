package siteaudit

import (
	"context"
)

func (service *Service) ListAudits(ctx context.Context, status *Status, action *Action, page, pageSize int32) (AuditPage, error) {
	return service.repository.ListAudits(ctx, status, action, page, pageSize)
}

func (service *Service) AuditDetail(ctx context.Context, auditID string) (Audit, error) {
	audit, err := service.repository.AuditDetail(ctx, auditID)
	if err != nil {
		return Audit{}, err
	}
	current := audit.CurrentSnapshot
	audit.CurrentSnapshot = current
	audit.EffectiveSnapshot, _ = MergeRequestedSnapshot(audit.BaseSnapshot, audit.ProposedSnapshot, current)
	var correction *Snapshot
	if audit.Status == StatusApproved {
		correction = &audit.FinalSnapshot
	} else if audit.ReviewDraftSnapshot != nil {
		correction = audit.ReviewDraftSnapshot
	}
	audit.Diff = BuildDiffViews(audit.BaseSnapshot, audit.ProposedSnapshot, current, correction)
	return audit, nil
}
