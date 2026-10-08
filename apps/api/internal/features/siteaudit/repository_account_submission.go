package siteaudit

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func (repository *Repository) createAccountSubmission(ctx context.Context, record submissionRecord) (string, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin account submission: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	bound := &Repository{pool: repository.pool, queries: repository.queries.WithTx(tx)}
	// Serialize owner create requests so simultaneous recommendations share one audit.
	if record.Action == ActionCreate {
		if err := bound.queries.LockOwnerSubmissionHost(ctx, record.Proposed.NormalizedHost); err != nil {
			return "", err
		}
	}
	var auditID string
	if record.Provenance.Channel == "OWNER_FRIEND_LINK" {
		pending, err := bound.queries.LockPendingCreateAuditForHost(ctx, record.Proposed.NormalizedHost)
		if err == nil {
			auditID, err = uuidString(pending.ID)
			if err != nil {
				return "", err
			}
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return "", err
		}
	}
	if auditID == "" {
		if record.Provenance.Channel == "OWNER_UPDATE" {
			if err := bound.validateOwnerAddress(ctx, record); err != nil {
				return "", err
			}
		}
		auditID, err = bound.insertSubmission(ctx, record)
		if err != nil {
			return "", accountWriteError(err)
		}
	}
	if record.Provenance.Channel == "OWNER_FRIEND_LINK" {
		if err := bound.addFriendRequest(ctx, auditID, record.Provenance, record.Input.Contact.NotifyByEmail); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return "", accountWriteError(err)
	}
	return auditID, nil
}

func (repository *Repository) addFriendRequest(ctx context.Context, auditID string, provenance submissionProvenance, notifyByEmail bool) error {
	userID, err := parseUUID(provenance.UserID)
	if err != nil {
		return err
	}
	sourceID, err := parseUUID(provenance.SourceSiteID)
	if err != nil {
		return err
	}
	ownerID, err := parseUUID(provenance.OwnershipID)
	if err != nil {
		return err
	}
	source, err := repository.queries.GetSiteByID(ctx, sourceID)
	if err != nil {
		return err
	}
	owner, err := repository.queries.LockCurrentSiteOwner(ctx, dbgen.LockCurrentSiteOwnerParams{ShortID: source.ShortID, UserID: userID})
	if err != nil || owner.ID != ownerID {
		return newServiceError("site_ownership_changed", http.StatusConflict, "site ownership changed; refresh the site")
	}
	id, err := parseUUID(auditID)
	if err != nil {
		return err
	}
	_, err = repository.queries.AddOwnerFriendRequest(ctx, dbgen.AddOwnerFriendRequestParams{AuditID: id, SourceSiteID: sourceID, UserID: userID, OwnershipID: ownerID, NotifyByEmail: notifyByEmail})
	return err
}

func (repository *Repository) validateOwnerAddress(ctx context.Context, record submissionRecord) error {
	userID, err := parseUUID(record.Provenance.UserID)
	if err != nil {
		return err
	}
	owner, err := repository.queries.LockCurrentSiteOwner(ctx, dbgen.LockCurrentSiteOwnerParams{ShortID: record.Base.ShortID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return newServiceError("site_ownership_changed", http.StatusConflict, "site ownership changed; refresh the site")
	}
	if err != nil {
		return err
	}
	ownerID, err := uuidString(owner.ID)
	if err != nil {
		return err
	}
	if ownerID != record.Provenance.OwnershipID {
		return newServiceError("site_ownership_changed", http.StatusConflict, "site ownership changed; refresh the site")
	}
	address := record.Proposed.Scheme + "://" + record.Proposed.NormalizedHost + record.Proposed.BasePath
	if address == owner.Address {
		return nil
	}
	verified, err := repository.queries.HasOwnerAddressProof(ctx, dbgen.HasOwnerAddressProofParams{SiteID: owner.SiteID, UserID: userID, Address: address})
	if err != nil {
		return err
	}
	if !verified {
		return newServiceError("verified_site_address_required", http.StatusConflict, "verify the new site address before submitting an update")
	}
	return nil
}

func accountWriteError(err error) error {
	if isOwnerConstraint(err) {
		return newServiceError("site_ownership_changed", http.StatusConflict, "site ownership or address verification changed; refresh the site")
	}
	return err
}

func ownerIdentity(owner dbgen.DirectorySiteOwnership) (submissionProvenance, error) {
	userID, err := uuidString(owner.UserID)
	if err != nil {
		return submissionProvenance{}, err
	}
	siteID, err := uuidString(owner.SiteID)
	if err != nil {
		return submissionProvenance{}, err
	}
	ownerID, err := uuidString(owner.ID)
	return submissionProvenance{UserID: userID, SourceSiteID: siteID, OwnershipID: ownerID}, err
}

func isOwnerConstraint(err error) bool {
	var databaseError *pgconn.PgError
	return errors.As(err, &databaseError) && (databaseError.Code == "P0001" || databaseError.ConstraintName == "site_ownership_changed" || databaseError.ConstraintName == "verified_site_address_required")
}
