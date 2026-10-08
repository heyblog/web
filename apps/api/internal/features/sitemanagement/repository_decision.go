package sitemanagement

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func (r *Repository) Decide(ctx context.Context, input Decision) (Claim, error) {
	err := r.transaction(ctx, func(q *dbgen.Queries) error {
		target, err := q.ClaimSite(ctx, input.Claim.ShortID)
		if err != nil {
			return repositoryError(err)
		}
		id, err := uuid(input.Claim.ID)
		if err != nil {
			return err
		}
		row, err := q.LockSiteClaim(ctx, id)
		if err != nil {
			return repositoryError(err)
		}
		claim := mapClaim(dbgen.GetSiteClaimRow(row))
		if claim.Status != "PENDING" {
			return failure("claim_finished", "This verification application is already finished", 409)
		}
		if claim.UserID == "" {
			return failure("claim_anonymized", "The applicant account is no longer available", 409)
		}
		if input.Status == "VERIFIED" && target.Visibility == "REMOVED" {
			return failure("site_removed", "Removed sites cannot be verified", 409)
		}
		if input.Status == "VERIFIED" && claim.Method != Manual && (claim.ExpiresAt == nil || !input.Now.Before(*claim.ExpiresAt)) {
			return failure("claim_expired", "This verification challenge has expired", 409)
		}
		siteID, err := uuid(claim.SiteID)
		if err != nil {
			return err
		}
		userID, err := uuid(claim.UserID)
		if err != nil {
			return err
		}
		actorID, err := uuid(input.ActorID)
		if err != nil {
			return err
		}
		consumed := false
		if input.Status == "VERIFIED" {
			if _, err := q.ResolveOwnershipUser(ctx, userID); err != nil {
				return repositoryError(err)
			}
			owner, ownerErr := q.GetSiteOwnership(ctx, siteID)
			if ownerErr == nil && owner.OUserID != claim.UserID {
				return failure("site_already_owned", "This site already has a verified owner", 409)
			}
			if ownerErr != nil && !errors.Is(ownerErr, pgx.ErrNoRows) {
				return ownerErr
			}
			if errors.Is(ownerErr, pgx.ErrNoRows) {
				if claim.Address != target.Address {
					return failure("address_changed", "The registered site address changed; create a new verification application", 409)
				}
				if err := q.CreateSiteOwnership(ctx, dbgen.CreateSiteOwnershipParams{Column1: siteID, Column2: userID, Address: claim.Address, Column4: id}); err != nil {
					return err
				}
				consumed = true
			} else if owner.Address != target.Address && claim.Address == target.Address {
				if err := q.RefreshSiteOwnership(ctx, dbgen.RefreshSiteOwnershipParams{Column1: siteID, Address: claim.Address, Column3: id}); err != nil {
					return err
				}
				consumed = true
			}
			if err := q.SiteOwnershipEvent(ctx, dbgen.SiteOwnershipEventParams{Column1: siteID, Column2: userID, Column3: actorID, Action: "VERIFIED", Reason: "Site control verified", Evidence: claim.Evidence}); err != nil {
				return err
			}
		}
		params := dbgen.FinishSiteClaimParams{Column1: id, Status: input.Status, Column3: actorID, Column5: consumed}
		if input.Reason != "" {
			params.ReviewReason = &input.Reason
		}
		changed, err := q.FinishSiteClaim(ctx, params)
		if err != nil {
			return err
		}
		if changed == 0 {
			return failure("claim_expired", "This verification challenge has expired", 409)
		}
		return nil
	})
	if err != nil {
		return Claim{}, err
	}
	return r.Claim(ctx, input.Claim.ID)
}
func (r *Repository) ChangeOwner(ctx context.Context, target Site, input OwnershipInput, actor string) error {
	return r.transaction(ctx, func(q *dbgen.Queries) error {
		current, err := q.ClaimSite(ctx, target.ShortID)
		if err != nil {
			return repositoryError(err)
		}
		siteID, err := uuid(current.ID)
		if err != nil {
			return err
		}
		actorID, err := uuid(actor)
		if err != nil {
			return err
		}
		owner, err := q.GetSiteOwnership(ctx, siteID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil && owner.OUserID == actor {
			return failure("self_review", "You cannot revoke your own site ownership", 403)
		}
		var userID pgtype.UUID
		action := "REVOKED"
		if input.UserID != "" {
			if current.Visibility == "REMOVED" {
				return failure("site_removed", "Removed sites cannot be assigned", 409)
			}
			userID, err = uuid(input.UserID)
			if err != nil {
				return err
			}
			if _, err := q.ResolveOwnershipUser(ctx, userID); err != nil {
				return repositoryError(err)
			}
			action = "REASSIGNED"
		} else if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else {
			userID, err = uuid(owner.OUserID)
			if err != nil {
				return err
			}
		}
		if err := q.DeleteSiteOwnership(ctx, siteID); err != nil {
			return err
		}
		if input.UserID != "" {
			if err := q.CreateSiteOwnership(ctx, dbgen.CreateSiteOwnershipParams{Column1: siteID, Column2: userID, Address: current.Address}); err != nil {
				return err
			}
		}
		return q.SiteOwnershipEvent(ctx, dbgen.SiteOwnershipEventParams{Column1: siteID, Column2: userID, Column3: actorID, Action: action, Reason: input.Reason, Evidence: &input.Evidence})
	})
}
