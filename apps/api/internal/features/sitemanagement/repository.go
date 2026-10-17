package sitemanagement

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	return Site{ID: row.ID, ShortID: row.ShortID, Name: row.Name, Address: row.Address, Visibility: row.Visibility}, nil
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
	return Ownership{ID: row.OID, SiteID: row.OSiteID, UserID: row.OUserID, Address: row.Address, Revision: row.Revision, Owner: Profile{ID: row.OUserID, Username: row.Username, DisplayName: row.DisplayName}}, nil
}
func mapClaim(row dbgen.GetSiteClaimRow) Claim {
	claim := Claim{SiteName: row.SiteName, CurrentAddress: row.CurrentAddress, Visibility: row.Visibility, OwnerUserID: row.OwnerUserID, OwnerAddress: row.OwnerAddress, ID: row.CID, SiteID: row.CSiteID, UserID: row.UserID, ShortID: row.ShortID, Address: row.Address, Method: Method(row.Method), Status: row.Status, Evidence: row.Evidence, EvidenceURL: row.EvidenceUrl, ReviewReason: row.ReviewReason, CreatedAt: row.CreatedAt.Time}
	if row.UserID != "" {
		claim.Applicant = &Profile{ID: row.UserID, Username: row.ApplicantUsername, DisplayName: row.ApplicantDisplayName}
	}
	if row.VerifiedAt.Valid {
		value := row.VerifiedAt.Time
		claim.VerifiedAt = &value
	}
	if row.ConsumedAt.Valid {
		value := row.ConsumedAt.Time
		claim.ConsumedAt = &value
	}
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
	claim := mapClaim(row)
	reviewedAt, err := r.queries.GetSiteClaimReviewedAt(ctx, id)
	if err != nil {
		return Claim{}, err
	}
	if reviewedAt.Valid {
		value := reviewedAt.Time
		claim.ReviewedAt = &value
	}
	metadata, err := r.queries.ListSiteClaimAttachmentMetadata(ctx, id)
	if err != nil {
		return Claim{}, err
	}
	claim.Attachments = make([]ClaimAttachment, 0, len(metadata))
	for _, attachment := range metadata {
		claim.AttachmentIDs = append(claim.AttachmentIDs, attachment.ID)
		claim.Attachments = append(claim.Attachments, attachmentFromRow(dbgen.GetSiteClaimAttachmentRow(attachment)))
	}
	return claim, nil
}
func (r *Repository) List(ctx context.Context, filter ClaimFilter) ([]Claim, error) {
	id, err := uuid(filter.UserID)
	if err != nil {
		return nil, err
	}
	siteID, err := uuid(filter.SiteID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListSiteClaims(ctx, dbgen.ListSiteClaimsParams{UserID: id, SiteID: siteID, Status: filter.Status, Method: filter.Method, PageLimit: filter.Limit, PageOffset: filter.Offset})
	if err != nil {
		return nil, err
	}
	claims := make([]Claim, 0, len(rows))
	for _, row := range rows {
		claims = append(claims, mapClaim(dbgen.GetSiteClaimRow(row)))
	}
	return claims, nil
}
func (r *Repository) transaction(ctx context.Context, operation func(*dbgen.Queries) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin ownership transaction: %w", err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if err := operation(r.queries.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *Repository) Create(ctx context.Context, input StoredClaim) (Claim, error) {
	var claimID string
	err := r.transaction(ctx, func(q *dbgen.Queries) error {
		userID, err := uuid(input.UserID)
		if err != nil {
			return err
		}
		if _, err := q.LockSiteClaimUser(ctx, userID); err != nil {
			return repositoryError(err)
		}
		target, err := q.LockClaimSite(ctx, input.Claim.ShortID)
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
		claimID, err = q.FindPendingSiteClaim(ctx, dbgen.FindPendingSiteClaimParams{Column1: siteID, Column2: userID, Address: input.Claim.Address, Method: string(input.Claim.Method)})
		if err == nil {
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if input.Claim.Method == Manual {
			counts, err := q.CountManualClaimWindows(ctx, userID)
			if err != nil {
				return err
			}
			if counts.Recent >= 3 {
				return failure("manual_submission_limit", "The manual verification submission limit has been reached", 429)
			}
			if counts.Pending >= 3 {
				return failure("manual_pending_limit", "You already have three manual verification applications awaiting review", 409)
			}
			rejected, err := q.HasRecentRejectedManualClaim(ctx, dbgen.HasRecentRejectedManualClaimParams{SiteID: siteID, UserID: userID})
			if err != nil {
				return err
			}
			if rejected {
				return failure("manual_rejected_cooldown", "Wait 24 hours after a rejected manual verification application", 409)
			}
		}
		owner, err := q.LockSiteOwnership(ctx, siteID)
		if err == nil && owner.OUserID != input.UserID {
			return failure("site_already_owned", "This site already has a verified owner", 409)
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil && owner.Address == target.Address && input.Claim.Address == target.Address {
			return failure("site_already_owned", "This address is already verified for your account", 409)
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
		if err != nil {
			return err
		}
		return bindClaimAttachments(ctx, q, claimID, input, siteID, userID)
	})
	if err != nil {
		return Claim{}, err
	}
	return r.Claim(ctx, claimID)
}

func bindClaimAttachments(ctx context.Context, q *dbgen.Queries, claimID string, input StoredClaim, siteID, userID pgtype.UUID) error {
	if len(input.AttachmentIDs) == 0 {
		return nil
	}
	if input.Claim.Method != Manual {
		return failure("manual_attachment_invalid", "Evidence attachments are only supported for manual verification", 422)
	}
	if err := validateAttachmentIDs(input.AttachmentIDs); err != nil {
		return err
	}
	claimUUID, err := uuid(claimID)
	if err != nil {
		return err
	}
	attachmentUUIDs := make([]pgtype.UUID, 0, len(input.AttachmentIDs))
	for _, rawID := range input.AttachmentIDs {
		attachmentID, err := uuid(rawID)
		if err != nil {
			return failure("manual_attachment_invalid", "Evidence attachment identifiers are invalid", 422)
		}
		attachmentUUIDs = append(attachmentUUIDs, attachmentID)
	}
	attachments, err := q.ValidateSiteClaimAttachments(ctx, dbgen.ValidateSiteClaimAttachmentsParams{UserID: userID, SiteID: siteID, Address: input.Claim.Address, Ids: attachmentUUIDs})
	if err != nil {
		return err
	}
	if len(attachments) != len(attachmentUUIDs) {
		return failure("attachment_not_ready", "The evidence attachment is not ready or belongs to another site", 409)
	}
	var totalBytes int64
	for _, attachment := range attachments {
		totalBytes += attachment.Size
	}
	if totalBytes > ManualAttachmentMaxBytes {
		return failure("manual_attachment_total_too_large", "Evidence attachments exceed the 20 MiB total limit", 422)
	}
	for _, attachmentID := range attachmentUUIDs {
		changed, err := q.BindSiteClaimAttachment(ctx, dbgen.BindSiteClaimAttachmentParams{ClaimID: claimUUID, ID: attachmentID, UserID: userID, SiteID: siteID, Address: input.Claim.Address})
		if err != nil {
			return err
		}
		if changed != 1 {
			return failure("attachment_not_ready", "The evidence attachment is not ready or belongs to another site", 409)
		}
	}
	return nil
}
