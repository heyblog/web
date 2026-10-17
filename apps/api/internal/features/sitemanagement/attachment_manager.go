package sitemanagement

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"time"

	cos "github.com/tencentyun/cos-go-sdk-v5"

	"heyblog-api/internal/platform/config"
)

type attachmentMetadataStore interface {
	AttachmentMetadata(context.Context, string) (ClaimAttachment, error)
	CreateAttachmentMetadata(context.Context, string, Site, AttachmentUploadInput) (ClaimAttachment, error)
	CompleteAttachmentMetadata(context.Context, string, string, func(ClaimAttachment) (EvidenceObject, error)) (ClaimAttachment, error)
	DeleteAttachmentMetadata(context.Context, string, string) error
}

// AttachmentManager joins durable database metadata with private object
// storage. A row is created before signing so the generated UUID determines
// the exact COS key and an abandoned upload remains cleanable.
type AttachmentManager struct {
	metadata attachmentMetadataStore
	storage  *EvidenceStorage
}

func NewAttachmentManager(metadata attachmentMetadataStore, configuration config.BackupCOSConfig) (*AttachmentManager, error) {
	storage, err := NewEvidenceStorage(configuration)
	if err != nil {
		return nil, err
	}
	return &AttachmentManager{metadata: metadata, storage: storage}, nil
}

func (manager *AttachmentManager) Storage() *EvidenceStorage {
	if manager == nil {
		return nil
	}
	return manager.storage
}

func (manager *AttachmentManager) InitializeAttachment(ctx context.Context, userID string, site Site, input AttachmentUploadInput) (ClaimAttachment, EvidenceUpload, error) {
	if err := input.Validate(); err != nil {
		return ClaimAttachment{}, EvidenceUpload{}, failure("manual_attachment_invalid", "The evidence attachment is invalid", 422)
	}
	if !manager.storage.Enabled() {
		return ClaimAttachment{}, EvidenceUpload{}, failure("attachment_storage_unavailable", "Evidence uploads are temporarily unavailable", 503)
	}
	input.StorageIdentity = manager.storage.origin
	attachment, err := manager.metadata.CreateAttachmentMetadata(ctx, userID, site, input)
	if err != nil {
		return ClaimAttachment{}, EvidenceUpload{}, err
	}
	upload, err := manager.storage.Sign(ctx, attachment.ID, input)
	if err != nil {
		_ = manager.metadata.DeleteAttachmentMetadata(ctx, userID, attachment.ID)
		if errors.Is(err, ErrEvidenceStorageUnavailable) {
			return ClaimAttachment{}, EvidenceUpload{}, failure("attachment_storage_unavailable", "Evidence uploads are temporarily unavailable", 503)
		}
		return ClaimAttachment{}, EvidenceUpload{}, err
	}
	return attachment, upload, nil
}

func (manager *AttachmentManager) CompleteAttachment(ctx context.Context, userID, id string, input AttachmentUploadInput) (ClaimAttachment, error) {
	return manager.metadata.CompleteAttachmentMetadata(ctx, userID, id, func(attachment ClaimAttachment) (EvidenceObject, error) {
		if attachment.StorageIdentity != manager.storage.origin {
			return EvidenceObject{}, failure("attachment_storage_unavailable", "Evidence uploads are temporarily unavailable", 503)
		}
		if input.ContentType != string(attachment.ContentType) || input.Size != attachment.Size {
			return EvidenceObject{}, failure("attachment_invalid", "The evidence attachment metadata does not match the upload", 422)
		}
		input.Filename = attachment.Filename
		object, err := manager.storage.Complete(ctx, id, input)
		if err != nil {
			if errors.Is(err, ErrEvidenceStorageUnavailable) {
				return EvidenceObject{}, failure("attachment_storage_unavailable", "Evidence uploads are temporarily unavailable", 503)
			}
			if errors.Is(err, errEvidenceObjectMissing) {
				return EvidenceObject{}, failure("attachment_not_ready", "The evidence attachment is not ready", 409)
			}
			return EvidenceObject{}, failure("attachment_invalid", "The evidence attachment could not be validated", 422)
		}
		return object, nil
	})
}

func (manager *AttachmentManager) DeleteAttachment(ctx context.Context, userID, id string) error {
	return manager.metadata.DeleteAttachmentMetadata(ctx, userID, id)
}

type AttachmentSettings struct {
	UploadOrigin string `json:"upload_origin"`
}
type EvidenceDownload struct {
	URL string `json:"url"`
}

func (manager *AttachmentManager) Settings() AttachmentSettings {
	return AttachmentSettings{UploadOrigin: manager.storage.origin}
}
func (manager *AttachmentManager) DownloadAttachment(ctx context.Context, userID, id string, reviewer bool) (EvidenceDownload, error) {
	attachment, err := manager.metadata.AttachmentMetadata(ctx, id)
	if err != nil {
		return EvidenceDownload{}, err
	}
	if attachment.UserID != userID && !reviewer {
		return EvidenceDownload{}, ErrNotFound
	}
	if attachment.ClaimID == "" || (attachment.Status != AttachmentBound && attachment.Status != AttachmentArchived) {
		return EvidenceDownload{}, ErrNotFound
	}
	key, err := manager.storage.Key(id)
	if err != nil || key != attachment.StorageKey || attachment.StorageIdentity != manager.storage.origin || !manager.storage.Enabled() {
		return EvidenceDownload{}, failure("attachment_storage_unavailable", "The evidence attachment is unavailable", 503)
	}
	signed, err := manager.storage.client.Object.GetPresignedURL2(ctx, http.MethodGet, key, time.Minute, &cos.ObjectGetOptions{
		ResponseContentType: string(attachment.ContentType), ResponseCacheControl: "private, no-store", ResponseContentDisposition: mime.FormatMediaType("attachment", map[string]string{"filename": attachment.Filename}),
	}, true)
	if err != nil {
		return EvidenceDownload{}, failure("attachment_storage_unavailable", "The evidence attachment is unavailable", 503)
	}
	return EvidenceDownload{URL: signed.String()}, nil
}
