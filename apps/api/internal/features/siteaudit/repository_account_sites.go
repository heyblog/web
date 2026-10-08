package siteaudit

import (
	"context"
	"errors"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"net/http"

	"github.com/jackc/pgx/v5"
)

func (repository *Repository) accountSites(ctx context.Context, user string) ([]Snapshot, error) {
	userID, err := parseUUID(user)
	if err != nil {
		return nil, err
	}
	rows, err := repository.queries.ListAccountSites(ctx, userID)
	if err != nil {
		return nil, err
	}
	items := make([]Snapshot, 0, len(rows))
	for _, row := range rows {
		item, err := loadSnapshot(ctx, repository.queries, row.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (repository *Repository) accountSite(ctx context.Context, user string, shortID string) (Snapshot, error) {
	userID, err := parseUUID(user)
	if err != nil {
		return Snapshot{}, err
	}
	row, err := repository.queries.GetAccountSite(ctx, dbgen.GetAccountSiteParams{ShortID: shortID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Snapshot{}, newServiceError("site_not_found", http.StatusNotFound, "the account site was not found")
	}
	if err != nil {
		return Snapshot{}, err
	}
	return loadSnapshot(ctx, repository.queries, row.ID)
}

func (repository *Repository) ownerProvenance(ctx context.Context, user string, shortID string) (submissionProvenance, error) {
	userID, err := parseUUID(user)
	if err != nil {
		return submissionProvenance{}, err
	}
	owner, err := repository.queries.GetCurrentSiteOwner(ctx, dbgen.GetCurrentSiteOwnerParams{ShortID: shortID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return submissionProvenance{}, newServiceError("site_ownership_required", http.StatusForbidden, "active site ownership is required")
	}
	if err != nil {
		return submissionProvenance{}, err
	}
	return ownerIdentity(owner)
}
func (repository *Repository) friendRequestID(ctx context.Context, audit string, provenance submissionProvenance) (string, error) {
	auditID, err := parseUUID(audit)
	if err != nil {
		return "", err
	}
	sourceID, err := parseUUID(provenance.SourceSiteID)
	if err != nil {
		return "", err
	}
	userID, err := parseUUID(provenance.UserID)
	if err != nil {
		return "", err
	}
	ownerID, err := parseUUID(provenance.OwnershipID)
	if err != nil {
		return "", err
	}
	id, err := repository.queries.GetOwnerFriendRequestForAudit(ctx, dbgen.GetOwnerFriendRequestForAuditParams{AuditID: auditID, SourceSiteID: sourceID, UserID: userID, OwnershipID: ownerID})
	if err != nil {
		return "", err
	}
	return uuidString(id)
}
