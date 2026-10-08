package siteaudit

import (
	"context"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"net/http"
)

func (repository *Repository) accountSubmissions(ctx context.Context, user string, page int32) ([]AuditListItem, error) {
	userID, err := parseUUID(user)
	if err != nil {
		return nil, err
	}
	rows, err := repository.queries.ListAccountAudits(ctx, dbgen.ListAccountAuditsParams{SubmitterUserID: userID, PageOffset: (page - 1) * 50})
	if err != nil {
		return nil, err
	}
	items := make([]AuditListItem, 0, len(rows))
	for _, row := range rows {
		audit, err := auditFromRow(row)
		if err != nil {
			return nil, err
		}
		items = append(items, AuditListItem{ID: audit.ID, Action: audit.Action, Status: audit.Status, SiteID: audit.SiteID,
			SiteName: audit.ProposedSnapshot.Name, SiteAddress: snapshotAddress(audit.ProposedSnapshot), CreatedAt: audit.CreatedAt,
			ReviewedAt: audit.ReviewedAt, SourceChannel: audit.SourceChannel, SubmitterUserID: audit.SubmitterUserID, SourceSiteID: audit.SourceSiteID})
	}
	return items, nil
}

func (repository *Repository) canReadAccountAudit(ctx context.Context, user, auditID string) (bool, error) {
	id, err := parseUUID(auditID)
	if err != nil {
		return false, newServiceError("audit_not_found", http.StatusNotFound, "the account submission was not found")
	}
	userID, err := parseUUID(user)
	if err != nil {
		return false, err
	}
	allowed, err := repository.queries.CanReadAccountAudit(ctx, dbgen.CanReadAccountAuditParams{ID: id, SubmitterUserID: userID})
	if err != nil {
		return false, err
	}
	return allowed, nil
}
