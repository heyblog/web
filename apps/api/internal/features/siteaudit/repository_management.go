package siteaudit

import (
	"context"
	"fmt"
	"math"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func (repository *Repository) ListAudits(ctx context.Context, status *Status, action *Action, page, pageSize int32) (AuditPage, error) {
	statusValue, actionValue := optionalFilters(status, action)
	offset := (page - 1) * pageSize
	rows, err := repository.queries.ListSiteAuditsForManagement(ctx, dbgen.ListSiteAuditsForManagementParams{Status: statusValue, Action: actionValue, PageOffset: offset, PageSize: pageSize})
	if err != nil {
		return AuditPage{}, fmt.Errorf("list site audits: %w", err)
	}
	total, err := repository.queries.CountSiteAuditsForManagement(ctx, dbgen.CountSiteAuditsForManagementParams{Status: statusValue, Action: actionValue})
	if err != nil {
		return AuditPage{}, fmt.Errorf("count site audits: %w", err)
	}
	items := make([]AuditListItem, 0, len(rows))
	for _, row := range rows {
		item, mapErr := mapAuditListItem(row)
		if mapErr != nil {
			return AuditPage{}, mapErr
		}
		items = append(items, item)
	}
	return AuditPage{Items: items, Page: page, PageSize: pageSize, TotalItems: total, TotalPages: int32(math.Ceil(float64(total) / float64(pageSize)))}, nil
}

func optionalFilters(status *Status, action *Action) (*string, *string) {
	var statusValue, actionValue *string
	if status != nil {
		value := string(*status)
		statusValue = &value
	}
	if action != nil {
		value := string(*action)
		actionValue = &value
	}
	return statusValue, actionValue
}

func (repository *Repository) AuditDetail(ctx context.Context, auditID string) (Audit, error) {
	id, err := parseUUID(auditID)
	if err != nil {
		return Audit{}, newServiceError("audit_not_found", 404, "the audit was not found")
	}
	row, err := repository.queries.GetSiteAuditByID(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return Audit{}, newServiceError("audit_not_found", 404, "the audit was not found")
		}
		return Audit{}, fmt.Errorf("get site audit: %w", err)
	}
	audit, err := auditFromRow(row)
	if err != nil {
		return Audit{}, err
	}
	current := audit.BaseSnapshot
	if row.SiteID.Valid {
		current, err = loadSnapshot(ctx, repository.queries, row.SiteID)
		if err != nil {
			return Audit{}, err
		}
		audit.HasCurrentSnapshot = true
	}
	audit.CurrentSnapshot = current
	return audit, nil
}
