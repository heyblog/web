package siteaudit

import (
	"fmt"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func auditFromRow(row dbgen.DirectorySiteAudit) (Audit, error) {
	audit := Audit{Action: Action(row.Action), Status: Status(row.Status), RequestReason: row.RequestReason, NotifyByEmail: row.NotifyByEmail}
	var err error
	if audit.ID, err = uuidString(row.ID); err != nil {
		return Audit{}, err
	}
	if row.SiteID.Valid {
		if audit.SiteID, err = uuidString(row.SiteID); err != nil {
			return Audit{}, err
		}
	}
	if row.BaseRevision != nil {
		audit.BaseRevision = *row.BaseRevision
	}
	if err := decodeSnapshot(row.BaseSnapshot, &audit.BaseSnapshot); err != nil {
		return Audit{}, fmt.Errorf("decode base snapshot: %w", err)
	}
	if err := decodeSnapshot(row.ProposedSnapshot, &audit.ProposedSnapshot); err != nil {
		return Audit{}, fmt.Errorf("decode proposed snapshot: %w", err)
	}
	if len(row.ReviewDraftSnapshot) > 0 {
		reviewDraft := Snapshot{}
		if err := decodeSnapshot(row.ReviewDraftSnapshot, &reviewDraft); err != nil {
			return Audit{}, fmt.Errorf("decode review draft snapshot: %w", err)
		}
		audit.ReviewDraftSnapshot = &reviewDraft
	}
	audit.ReviewDraftRevision = row.ReviewDraftRevision
	if row.ReviewDraftUpdatedBy.Valid {
		audit.ReviewDraftUpdatedBy, _ = uuidString(row.ReviewDraftUpdatedBy)
	}
	if row.ReviewDraftUpdatedAt.Valid {
		updatedAt := row.ReviewDraftUpdatedAt.Time
		audit.ReviewDraftUpdatedAt = &updatedAt
	}
	if err := decodeSnapshot(row.FinalSnapshot, &audit.FinalSnapshot); err != nil {
		return Audit{}, fmt.Errorf("decode final snapshot: %w", err)
	}
	audit.SubmitterName = stringValue(row.SubmitterName)
	audit.SubmitterEmail = stringValue(row.SubmitterEmail)
	audit.ReviewerComment = stringValue(row.ReviewerComment)
	if row.ReviewedBy.Valid {
		audit.ReviewedBy, _ = uuidString(row.ReviewedBy)
	}
	if row.ReviewedAt.Valid {
		reviewedAt := row.ReviewedAt.Time
		audit.ReviewedAt = &reviewedAt
	}
	audit.CreatedAt = row.CreatedAt.Time
	audit.UpdatedAt = row.UpdatedAt.Time
	return audit, nil
}

func mapAuditListItem(row dbgen.ListSiteAuditsForManagementRow) (AuditListItem, error) {
	id, err := uuidString(row.ID)
	if err != nil {
		return AuditListItem{}, err
	}
	proposed := Snapshot{}
	if err := decodeSnapshot(row.ProposedSnapshot, &proposed); err != nil {
		return AuditListItem{}, fmt.Errorf("decode audit list snapshot: %w", err)
	}
	item := AuditListItem{ID: id, Action: Action(row.Action), Status: Status(row.Status), SiteName: proposed.Name, SiteAddress: snapshotAddress(proposed), SubmitterName: stringValue(row.SubmitterName), SubmitterEmail: stringValue(row.SubmitterEmail), ReviewedAt: timestampPointer(row.ReviewedAt), CreatedAt: row.CreatedAt.Time}
	if row.SiteID.Valid {
		item.SiteID, err = uuidString(row.SiteID)
	}
	return item, err
}
