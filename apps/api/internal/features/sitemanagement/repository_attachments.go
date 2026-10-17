package sitemanagement

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type AttachmentCleanupItem struct {
	ID              string
	Status          AttachmentStatus
	StorageKey      string
	StorageIdentity string
	CleanupUpload   bool
}

func attachmentFromRow(row dbgen.GetSiteClaimAttachmentRow) ClaimAttachment {
	attachment := ClaimAttachment{UserID: row.UserID, ID: row.ID, ClaimID: row.ClaimID, Filename: row.Filename, ContentType: AttachmentKind(row.ContentType), Size: row.Size, Status: AttachmentStatus(row.Status), StorageKey: row.StorageKey, StorageIdentity: row.StorageIdentity, CreatedAt: row.CreatedAt.Time}
	if row.Sha256 != nil {
		attachment.SHA256 = *row.Sha256
	}
	return attachment
}

func (r *Repository) CreateAttachmentMetadata(ctx context.Context, userID string, target Site, input AttachmentUploadInput) (ClaimAttachment, error) {
	if err := input.Validate(); err != nil {
		return ClaimAttachment{}, failure("manual_attachment_invalid", "The evidence attachment is invalid", 422)
	}
	accountID, err := uuid(userID)
	if err != nil {
		return ClaimAttachment{}, err
	}
	siteID, err := uuid(target.ID)
	if err != nil {
		return ClaimAttachment{}, err
	}
	var attachmentID string
	err = r.transaction(ctx, func(q *dbgen.Queries) error {
		if _, err := q.LockSiteClaimUser(ctx, accountID); err != nil {
			return repositoryError(err)
		}
		count, err := q.CountRecentSiteClaimAttachments(ctx, accountID)
		if err != nil {
			return err
		}
		if count >= 18 {
			return failure("manual_attachment_limit", "The evidence upload limit has been reached", 429)
		}
		attachmentID, err = q.CreateSiteClaimAttachment(ctx, dbgen.CreateSiteClaimAttachmentParams{SiteID: siteID, UserID: accountID, Address: target.Address, Filename: input.Filename, ContentType: string(input.kindValue()), Size: input.Size, StorageKey: "pending", StorageIdentity: input.StorageIdentity})
		if err != nil {
			return err
		}
		return q.SetSiteClaimAttachmentStorageKey(ctx, dbgen.SetSiteClaimAttachmentStorageKeyParams{ID: attachmentUUID(attachmentID), StorageKey: evidenceStoragePrefix + attachmentID})
	})
	if err != nil {
		return ClaimAttachment{}, err
	}
	return ClaimAttachment{ID: attachmentID, Filename: input.Filename, ContentType: input.kindValue(), Size: input.Size, Status: AttachmentUploading}, nil
}

func (r *Repository) CompleteAttachmentMetadata(ctx context.Context, userID, rawID string, complete func(ClaimAttachment) (EvidenceObject, error)) (ClaimAttachment, error) {
	accountID, err := uuid(userID)
	if err != nil {
		return ClaimAttachment{}, err
	}
	attachmentID, err := uuid(rawID)
	if err != nil {
		return ClaimAttachment{}, ErrNotFound
	}
	var result ClaimAttachment
	err = r.transaction(ctx, func(q *dbgen.Queries) error {
		row, err := q.LockSiteClaimAttachment(ctx, attachmentID)
		if err != nil {
			return repositoryError(err)
		}
		if row.UserID != userID {
			return ErrNotFound
		}
		if row.Status == string(AttachmentReady) {
			result = attachmentFromRow(dbgen.GetSiteClaimAttachmentRow(row))
			return nil
		}
		if row.Status != string(AttachmentUploading) {
			return failure("attachment_not_ready", "The evidence attachment cannot be completed", 409)
		}
		if time.Since(row.CreatedAt.Time) >= 15*time.Minute {
			return failure("attachment_not_ready", "The evidence upload has expired", 409)
		}
		object, err := complete(attachmentFromRow(dbgen.GetSiteClaimAttachmentRow(row)))
		if err != nil {
			return err
		}
		if object.Key != row.StorageKey {
			return failure("attachment_invalid", "The evidence attachment is invalid", 422)
		}
		changed, err := q.CompleteSiteClaimAttachment(ctx, dbgen.CompleteSiteClaimAttachmentParams{ID: attachmentID, UserID: accountID, ContentType: string(object.ContentType), Size: object.Size, Sha256: &object.SHA256})
		if err != nil {
			return err
		}
		if changed != 1 {
			return failure("attachment_not_ready", "The evidence attachment cannot be completed", 409)
		}
		updated, err := q.GetSiteClaimAttachment(ctx, attachmentID)
		if err != nil {
			return err
		}
		result = attachmentFromRow(updated)
		return nil
	})
	if err != nil {
		return ClaimAttachment{}, err
	}
	return result, nil
}

func (r *Repository) DeleteAttachmentMetadata(ctx context.Context, userID, rawID string) error {
	accountID, err := uuid(userID)
	if err != nil {
		return ErrNotFound
	}
	attachmentID, err := uuid(rawID)
	if err != nil {
		return ErrNotFound
	}
	return r.transaction(ctx, func(q *dbgen.Queries) error {
		changed, err := q.DeleteSiteClaimAttachment(ctx, dbgen.DeleteSiteClaimAttachmentParams{ID: attachmentID, UserID: accountID})
		if err != nil {
			return err
		}
		if changed == 0 {
			row, getErr := q.GetSiteClaimAttachment(ctx, attachmentID)
			if getErr != nil {
				return repositoryError(getErr)
			}
			if row.UserID != userID {
				return ErrNotFound
			}
			switch AttachmentStatus(row.Status) {
			case AttachmentCleanupPending, AttachmentDeleted:
				return nil
			case AttachmentBound, AttachmentArchived:
				return failure("attachment_bound", "Evidence already belongs to a submitted claim", 409)
			}
		}
		return nil
	})
}

func (input AttachmentUploadInput) kindValue() AttachmentKind {
	kind, _ := input.kind()
	return kind
}

var _ attachmentMetadataStore = (*Repository)(nil)

func (r *Repository) ListAttachmentCleanup(ctx context.Context) ([]AttachmentCleanupItem, error) {
	if err := r.queries.ExpireSiteClaimAttachments(ctx); err != nil {
		return nil, err
	}
	rows, err := r.queries.ListSiteClaimAttachmentCleanup(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]AttachmentCleanupItem, 0, len(rows))
	for _, row := range rows {
		result = append(result, AttachmentCleanupItem{ID: row.ID, Status: AttachmentStatus(row.Status), StorageKey: row.StorageKey, StorageIdentity: row.StorageIdentity, CleanupUpload: row.CleanupUpload})
	}
	return result, nil
}

func (r *Repository) MarkAttachmentArchived(ctx context.Context, id string) error {
	_, err := r.queries.MarkSiteClaimAttachmentArchived(ctx, attachmentUUID(id))
	return err
}

func (r *Repository) MarkAttachmentDeleted(ctx context.Context, id string) error {
	_, err := r.queries.MarkSiteClaimAttachmentDeleted(ctx, attachmentUUID(id))
	return err
}

func attachmentUUID(raw string) pgtype.UUID { value, _ := uuid(raw); return value }
func (r *Repository) MarkAttachmentUploadCleaned(ctx context.Context, id string) error {
	return r.queries.MarkSiteClaimAttachmentUploadCleaned(ctx, attachmentUUID(id))
}

func (r *Repository) AttachmentMetadata(ctx context.Context, id string) (ClaimAttachment, error) {
	parsed, err := uuid(id)
	if err != nil {
		return ClaimAttachment{}, ErrNotFound
	}
	row, err := r.queries.GetSiteClaimAttachment(ctx, parsed)
	if err != nil {
		return ClaimAttachment{}, repositoryError(err)
	}
	return attachmentFromRow(row), nil
}
