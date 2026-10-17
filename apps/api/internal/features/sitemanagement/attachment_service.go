package sitemanagement

import (
	"context"
	"errors"

	"heyblog-api/internal/domain/site"
	"heyblog-api/internal/features/auth"
)

func (s *Service) InitializeAttachment(ctx context.Context, user auth.User, input AttachmentInitializeInput) (AttachmentUploadResult, error) {
	if s.attachments == nil {
		return AttachmentUploadResult{}, failure("attachment_storage_unavailable", "Evidence uploads are temporarily unavailable", 503)
	}
	target, err := s.store.Site(ctx, input.ShortID)
	if err != nil {
		return AttachmentUploadResult{}, err
	}
	if target.Visibility == "REMOVED" {
		return AttachmentUploadResult{}, failure("site_removed", "Removed sites cannot be verified", 409)
	}
	address := target.Address
	if input.Address != "" {
		parsed, parseErr := site.NormalizeAddress(input.Address)
		if parseErr != nil {
			return AttachmentUploadResult{}, failure("invalid_address", "The site address is invalid", 422)
		}
		address = parsed.CanonicalURL()
	}
	target.Address = address
	attachment, upload, err := s.attachments.InitializeAttachment(ctx, user.ID, target, AttachmentUploadInput{Filename: input.Filename, ContentType: input.ContentType, Size: input.Size})
	if err != nil {
		return AttachmentUploadResult{}, err
	}
	maxSize := ManualImageMaxBytes
	if attachment.ContentType == AttachmentPDF {
		maxSize = ManualPDFMaxBytes
	}
	return AttachmentUploadResult{Attachment: attachment, UploadURL: upload.URL, UploadHeaders: upload.Headers, ExpiresAt: upload.ExpiresAt, MaxSize: maxSize, ContentType: attachment.ContentType}, nil
}

func (s *Service) CompleteAttachment(ctx context.Context, user auth.User, id string, input AttachmentCompleteInput) (ClaimAttachment, error) {
	if s.attachments == nil {
		return ClaimAttachment{}, failure("attachment_storage_unavailable", "Evidence uploads are temporarily unavailable", 503)
	}
	result, err := s.attachments.CompleteAttachment(ctx, user.ID, id, AttachmentUploadInput{ContentType: input.ContentType, Size: input.Size, SHA256: input.SHA256})
	if errors.Is(err, ErrEvidenceStorageUnavailable) {
		return ClaimAttachment{}, failure("attachment_storage_unavailable", "Evidence uploads are temporarily unavailable", 503)
	}
	return result, err
}

func (s *Service) DeleteAttachment(ctx context.Context, user auth.User, id string) error {
	if s.attachments == nil {
		return failure("attachment_storage_unavailable", "Evidence uploads are temporarily unavailable", 503)
	}
	return s.attachments.DeleteAttachment(ctx, user.ID, id)
}
