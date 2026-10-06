package siteaudit

import (
	"context"
	"net/http"

	"heyblog-api/internal/features/auth"
)

type reviewContext struct {
	Reviewer      auth.User
	Input         ReviewInput
	GeneratedTags []TagSnapshot
}

func (service *Service) approve(ctx context.Context, transaction AuditTransaction, state reviewContext) (Audit, error) {
	audit, err := transaction.ReadLockedAudit(ctx)
	if err != nil {
		return Audit{}, err
	}
	reviewer, input := state.Reviewer, state.Input
	current := audit.BaseSnapshot
	if audit.Action != ActionCreate {
		current, err = transaction.LockSiteSnapshot(ctx, audit.SiteID)
		if err != nil {
			return Audit{}, err
		}
		if input.ExpectedSiteRevision != current.Revision {
			return Audit{}, newServiceError("site_revision_changed", http.StatusConflict, "the site changed; refresh the three-way diff before reviewing")
		}
	}
	final, conflicts := MergeRequestedSnapshot(audit.BaseSnapshot, audit.ProposedSnapshot, current)
	if audit.ReviewDraftSnapshot != nil {
		final = *audit.ReviewDraftSnapshot
	}
	if len(conflicts) > 0 && audit.ReviewDraftSnapshot == nil {
		return Audit{}, newServiceError("audit_conflicts_unresolved", http.StatusConflict, "the three-way diff contains unresolved conflicts")
	}
	for i, tag := range final.Tags {
		if tag.ID != "" || tag.Slug != "" {
			continue
		}
		for _, generated := range state.GeneratedTags {
			if generated.SuggestedName == tag.SuggestedName && generated.Description == tag.Description {
				final.Tags[i].Slug = generated.Slug
				break
			}
		}
	}
	if audit.Action == ActionCreate || audit.Action == ActionUpdate {
		if err := validateSnapshotLocations(final); err != nil {
			return Audit{}, err
		}
	}
	createsProgramDependencies := false
	for _, component := range final.Components {
		if component.Role == "SITE_PROGRAM" && component.ID == "" {
			createsProgramDependencies = true
			break
		}
	}

	final, err = transaction.ResolveTaxonomy(ctx, reviewer, final, current)
	if err != nil {
		return Audit{}, err
	}
	final, err = transaction.ApplySite(ctx, reviewedSite{
		Action: audit.Action, Current: current, Final: final, SiteID: audit.SiteID, ReviewerID: reviewer.ID,
		CreatesProgramDependencies: createsProgramDependencies,
	}, service.newShortID)
	if err != nil {
		return Audit{}, err
	}
	return transaction.Approve(ctx, decisionRecord{AuditID: audit.ID, ReviewerID: reviewer.ID, ReviewerComment: input.ReviewerComment, Final: final})
}
