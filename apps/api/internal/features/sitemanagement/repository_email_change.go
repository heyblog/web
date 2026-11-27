package sitemanagement

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func (r *Repository) CloseInactiveEmailChangeClaims(ctx context.Context) error {
	return r.queries.CloseInactiveEmailChangeSiteClaims(ctx)
}

func requireSiteEmailChange(ctx context.Context, q *dbgen.Queries, requestID, userID pgtype.UUID) error {
	request, err := q.GetSiteEmailChangeRequest(ctx, dbgen.GetSiteEmailChangeRequestParams{
		RequestID: requestID, UserID: userID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return failure("email_change_missing", "The email change request is unavailable", 404)
	}
	if err != nil {
		return err
	}
	if !request.Active {
		return failure("email_change_expired", "The email change request has expired", 409)
	}
	return nil
}

func requireEmailChangeOwnership(ctx context.Context, q *dbgen.Queries, claim Claim, address string) error {
	if address != claim.Address {
		return failure("email_change_site_changed", "The registered site address has changed", 409)
	}
	siteID, err := uuid(claim.SiteID)
	if err != nil {
		return err
	}
	owner, err := q.LockSiteOwnership(ctx, siteID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (owner.OUserID != claim.UserID || owner.OID != claim.EmailChangeOwnershipID)) {
		return failure("email_change_site_unavailable", "The site ownership has changed", 409)
	}
	return err
}

func lockSiteClaimApplicant(ctx context.Context, q *dbgen.Queries, claim Claim) error {
	userID, err := uuid(claim.UserID)
	if err != nil {
		return err
	}
	if _, err := q.LockSiteClaimUser(ctx, userID); err != nil {
		return repositoryError(err)
	}
	if claim.EmailChangeID == "" {
		return nil
	}
	requestID, err := uuid(claim.EmailChangeID)
	if err != nil {
		return err
	}
	return requireSiteEmailChange(ctx, q, requestID, userID)
}
