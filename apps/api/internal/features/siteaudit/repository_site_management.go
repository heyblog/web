package siteaudit

import (
	"context"
	"fmt"

	"heyblog-api/internal/domain/site"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

const managedSiteMaximumPages int64 = 1_000_000

func (repository *Repository) ListManagedSites(ctx context.Context, query, visibility string, page, pageSize int32) (ManagedSitePage, error) {
	total, err := repository.queries.CountSitesForManagement(ctx, dbgen.CountSitesForManagementParams{
		Visibility: visibility,
		QueryText:  query,
	})
	if err != nil {
		return ManagedSitePage{}, fmt.Errorf("count sites for management: %w", err)
	}
	totalPages := int32(1)
	if total > 0 {
		pageCount := (total + int64(pageSize) - 1) / int64(pageSize)
		if pageCount > managedSiteMaximumPages {
			pageCount = managedSiteMaximumPages
		}
		totalPages = int32(pageCount) // #nosec G115 -- pageCount is explicitly bounded.
	}
	if page > totalPages {
		page = totalPages
	}
	rows, err := repository.queries.ListSitesForManagement(ctx, dbgen.ListSitesForManagementParams{
		Visibility: visibility,
		QueryText:  query,
		PageSize:   pageSize,
		PageOffset: (page - 1) * pageSize,
	})
	if err != nil {
		return ManagedSitePage{}, fmt.Errorf("list sites for management: %w", err)
	}
	items := make([]ManagedSite, 0, len(rows))
	for _, row := range rows {
		urlValue, mapErr := (site.Address{
			Scheme:         row.Scheme,
			NormalizedHost: row.NormalizedHost,
			BasePath:       row.BasePath,
		}).HomepageURL()
		if mapErr != nil {
			return ManagedSitePage{}, fmt.Errorf("map managed site address: %w", mapErr)
		}
		item := ManagedSite{
			ShortID: row.ShortID, CustomID: stringValue(row.CustomID), Name: row.Name, URL: urlValue,
			AccessScope: row.AccessScope, Visibility: row.Visibility,
			VisibilityReason: stringValue(row.VisibilityReason), Revision: row.Revision,
			JoinedAt: row.JoinedAt.Time, UpdatedAt: row.UpdatedAt.Time,
		}
		if row.PendingAuditID.Valid {
			pendingAuditID, idErr := uuidString(row.PendingAuditID)
			if idErr != nil {
				return ManagedSitePage{}, fmt.Errorf("map managed site pending audit: %w", idErr)
			}
			item.PendingAudit = &ManagedSiteAudit{ID: pendingAuditID, Action: Action(stringValue(row.PendingAuditAction))}
		}
		items = append(items, item)
	}
	return ManagedSitePage{
		Items: items, Page: page, PageSize: pageSize, TotalItems: total, TotalPages: totalPages,
	}, nil
}
