package sitemanagement

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type Repository struct {
	pool    *pgxpool.Pool
	queries *dbgen.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: dbgen.New(pool)}
}
func uuid(raw string) (pgtype.UUID, error) {
	var result pgtype.UUID
	if raw == "" {
		return result, nil
	}
	if err := result.Scan(raw); err != nil {
		return result, ErrNotFound
	}
	return result, nil
}
func repositoryError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
func (r *Repository) Site(ctx context.Context, shortID string) (Site, error) {
	row, err := r.queries.ClaimSite(ctx, shortID)
	if err != nil {
		return Site{}, repositoryError(err)
	}
	return Site{ID: row.ID, ShortID: row.ShortID, Address: row.Address, Visibility: row.Visibility}, nil
}
func (r *Repository) Owner(ctx context.Context, siteID string) (Ownership, error) {
	id, err := uuid(siteID)
	if err != nil {
		return Ownership{}, err
	}
	row, err := r.queries.GetSiteOwnership(ctx, id)
	if err != nil {
		return Ownership{}, repositoryError(err)
	}
	return Ownership{ID: row.OID, SiteID: row.OSiteID, UserID: row.OUserID, Address: row.Address, Revision: row.Revision}, nil
}
func mapClaim(row dbgen.GetSiteClaimRow) Claim {
	claim := Claim{ID: row.CID, SiteID: row.CSiteID, UserID: row.UserID, ShortID: row.ShortID, Address: row.Address, Method: Method(row.Method), Status: row.Status, Evidence: row.Evidence, EvidenceURL: row.EvidenceUrl, ReviewReason: row.ReviewReason, CreatedAt: row.CreatedAt.Time}
	if row.TokenHash != nil {
		claim.TokenHash = *row.TokenHash
	}
	if row.ExpiresAt.Valid {
		expiry := row.ExpiresAt.Time
		claim.ExpiresAt = &expiry
	}
	return claim
}
func (r *Repository) Claim(ctx context.Context, raw string) (Claim, error) {
	id, err := uuid(raw)
	if err != nil {
		return Claim{}, err
	}
	row, err := r.queries.GetSiteClaim(ctx, id)
	if err != nil {
		return Claim{}, repositoryError(err)
	}
	return mapClaim(row), nil
}
func (r *Repository) List(ctx context.Context, userID string, limit, offset int32) ([]Claim, error) {
	id, err := uuid(userID)
	if err != nil {
		return nil, err
	}
	ids, err := r.queries.ListSiteClaims(ctx, dbgen.ListSiteClaimsParams{Limit: limit, Offset: offset, UserID: id})
	if err != nil {
		return nil, err
	}
	claims := make([]Claim, 0, len(ids))
	for _, id := range ids {
		claim, err := r.Claim(ctx, id)
		if err != nil {
			return nil, err
		}
		claims = append(claims, claim)
	}
	return claims, nil
}
func (r *Repository) transaction(ctx context.Context, operation func(*dbgen.Queries) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin ownership transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := operation(r.queries.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *Repository) Create(ctx context.Context, input StoredClaim) (Claim, error) {
	var claimID string
	err := r.transaction(ctx, func(q *dbgen.Queries) error {
		target, err := q.ClaimSite(ctx, input.Claim.ShortID)
		if err != nil {
			return repositoryError(err)
		}
		if target.Visibility == "REMOVED" {
			return failure("site_removed", "Removed sites cannot be verified", 409)
		}
		siteID, err := uuid(target.ID)
		if err != nil {
			return err
		}
		userID, err := uuid(input.UserID)
		if err != nil {
			return err
		}
		owner, err := q.GetSiteOwnership(ctx, siteID)
		if err == nil && owner.OUserID != input.UserID {
			return failure("site_already_owned", "This site already has a verified owner", 409)
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if errors.Is(err, pgx.ErrNoRows) && input.Claim.Address != target.Address {
			return failure("address_owner_required", "Verify the existing site address before changing it", 403)
		}
		params := dbgen.CreateSiteClaimParams{Column1: siteID, Column2: userID, Address: input.Claim.Address, Method: string(input.Claim.Method), Evidence: input.Claim.Evidence, EvidenceUrl: input.Claim.EvidenceURL}
		if input.Claim.TokenHash != "" {
			params.TokenHash = &input.Claim.TokenHash
		}
		if input.Claim.ExpiresAt != nil {
			params.ExpiresAt = pgtype.Timestamptz{Time: *input.Claim.ExpiresAt, Valid: true}
		}
		claimID, err = q.CreateSiteClaim(ctx, params)
		return err
	})
	if err != nil {
		return Claim{}, err
	}
	return r.Claim(ctx, claimID)
}
