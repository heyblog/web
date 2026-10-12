package sitemanagement

import (
	"context"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func (r *Repository) Regenerate(ctx context.Context, input Claim, tokenHash string, expires time.Time) (Claim, error) {
	err := r.transaction(ctx, func(q *dbgen.Queries) error {
		target, err := q.LockClaimSite(ctx, input.ShortID)
		if err != nil {
			return repositoryError(err)
		}
		id, err := uuid(input.ID)
		if err != nil {
			return err
		}
		row, err := q.LockSiteClaim(ctx, id)
		if err != nil {
			return repositoryError(err)
		}
		claim := mapClaim(dbgen.GetSiteClaimRow(row))
		if claim.UserID != input.UserID {
			return ErrNotFound
		}
		if claim.TokenHash != input.TokenHash {
			return failure("proof_changed", "The verification proof changed; reload the application", 409)
		}
		if claim.Status != "PENDING" || claim.Method == Manual {
			return failure("claim_finished", "This application cannot issue a new challenge", 409)
		}
		if target.Visibility == "REMOVED" {
			return failure("site_removed", "Removed sites cannot be verified", 409)
		}
		if claim.OwnerUserID != "" && claim.OwnerUserID != claim.UserID {
			return failure("site_already_owned", "This site already has a verified owner", 409)
		}
		if claim.OwnerUserID == "" && claim.Address != target.Address {
			return failure("address_changed", "The registered address changed; create a new application", 409)
		}
		return q.RegenerateSiteClaim(ctx, dbgen.RegenerateSiteClaimParams{Column1: id, TokenHash: &tokenHash, ExpiresAt: pgtype.Timestamptz{Time: expires, Valid: true}})
	})
	if err != nil {
		return Claim{}, err
	}
	return r.Claim(ctx, input.ID)
}

func (r *Repository) History(ctx context.Context, siteID string) ([]OwnershipEvent, error) {
	id, err := uuid(siteID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListSiteOwnershipEvents(ctx, id)
	if err != nil {
		return nil, err
	}
	result := make([]OwnershipEvent, 0, len(rows))
	for _, row := range rows {
		event := OwnershipEvent{ID: row.ID, Action: row.Action, Reason: row.Reason, Evidence: row.Evidence, CreatedAt: row.CreatedAt.Time}
		if row.UserID != "" {
			value := row.UserID
			event.UserID = &value
		}
		if row.ActorID != "" {
			value := row.ActorID
			event.ActorID = &value
		}
		result = append(result, event)
	}
	return result, nil
}
