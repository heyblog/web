package sitemanagement

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"mime"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	ManualEvidenceMinRunes          = 20
	ManualEvidenceMaxRunes          = 4000
	ManualAttachmentMaxCount        = 3
	ManualAttachmentMaxBytes  int64 = 20 << 20
	ManualImageMaxBytes       int64 = 5 << 20
	ManualPDFMaxBytes         int64 = 10 << 20
	ManualAttachmentMaxPixels int64 = 20_000_000
)

type AttachmentKind string

const (
	AttachmentPNG  AttachmentKind = "image/png"
	AttachmentJPEG AttachmentKind = "image/jpeg"
	AttachmentPDF  AttachmentKind = "application/pdf"
)

type AttachmentStatus string

const (
	AttachmentUploading      AttachmentStatus = "UPLOADING"
	AttachmentReady          AttachmentStatus = "READY"
	AttachmentBound          AttachmentStatus = "BOUND"
	AttachmentCleanupPending AttachmentStatus = "CLEANUP_PENDING"
	AttachmentArchived       AttachmentStatus = "ARCHIVED"
	AttachmentDeleted        AttachmentStatus = "DELETED"
)

// ClaimAttachment is the API-facing metadata for one private evidence object.
// Content is deliberately absent; callers receive it through an authenticated
// preview/download operation owned by the storage implementation.
type ClaimAttachment struct {
	UserID          string           `json:"-"`
	ID              string           `json:"id"`
	ClaimID         string           `json:"claim_id,omitempty"`
	Filename        string           `json:"filename"`
	ContentType     AttachmentKind   `json:"content_type"`
	Size            int64            `json:"size"`
	SHA256          string           `json:"sha256,omitempty"`
	Status          AttachmentStatus `json:"status"`
	StorageKey      string           `json:"-"`
	StorageIdentity string           `json:"-"`
	CreatedAt       time.Time        `json:"-"`
	UploadURL       string           `json:"upload_url,omitempty"`
}

// AttachmentUploadInput is the metadata sent before a direct upload. The
// completed bytes must be checked with ValidateAttachmentContent as well.
type AttachmentUploadInput struct {
	Filename        string
	ContentType     string
	Size            int64
	SHA256          string
	StorageIdentity string
}

func (input AttachmentUploadInput) kind() (AttachmentKind, error) {
	contentType := strings.ToLower(strings.TrimSpace(input.ContentType))
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = mediaType
	}
	var kind AttachmentKind
	switch contentType {
	case string(AttachmentPNG):
		kind = AttachmentPNG
	case string(AttachmentJPEG):
		kind = AttachmentJPEG
	case string(AttachmentPDF):
		kind = AttachmentPDF
	default:
		return "", errors.New("unsupported attachment content type")
	}
	if input.Filename == "" || filepath.Base(input.Filename) != input.Filename || input.Filename == "." || input.Filename == ".." || len([]rune(input.Filename)) > 255 || strings.ContainsAny(input.Filename, "\\\x00\r\n") {
		return "", errors.New("attachment filename is invalid")
	}
	if input.Size <= 0 {
		return "", errors.New("attachment size must be positive")
	}
	max := ManualPDFMaxBytes
	if kind != AttachmentPDF {
		max = ManualImageMaxBytes
	}
	if input.Size > max {
		return "", fmt.Errorf("attachment exceeds %d byte limit", max)
	}
	return kind, nil
}

func (input AttachmentUploadInput) Validate() error {
	_, err := input.kind()
	return err
}

// ValidateAttachmentContent verifies the declared type against the bytes at
// the upload completion boundary. PDF data is never rendered as HTML.
func ValidateAttachmentContent(input AttachmentUploadInput, content []byte) error {
	kind, err := input.kind()
	if err != nil {
		return err
	}
	if int64(len(content)) != input.Size {
		return errors.New("attachment size does not match uploaded content")
	}
	switch kind {
	case AttachmentPDF:
		if len(content) < len("%PDF-") || !bytes.Equal(content[:len("%PDF-")], []byte("%PDF-")) {
			return errors.New("PDF attachment header is invalid")
		}
	case AttachmentPNG, AttachmentJPEG:
		config, format, decodeErr := image.DecodeConfig(bytes.NewReader(content))
		if decodeErr != nil {
			return errors.New("image attachment data is invalid")
		}
		if (kind == AttachmentPNG && format != "png") || (kind == AttachmentJPEG && format != "jpeg") {
			return errors.New("image attachment type does not match content")
		}
		if config.Width <= 0 || config.Height <= 0 || int64(config.Width) > ManualAttachmentMaxPixels/int64(config.Height) {
			return errors.New("image attachment dimensions are too large")
		}
	default:
		return errors.New("unsupported attachment content type")
	}
	return nil
}

func validateManualEvidence(evidence string) (string, error) {
	trimmed := strings.TrimSpace(evidence)
	if !utf8.ValidString(trimmed) {
		return "", failure("invalid_evidence", "Ownership evidence must be valid UTF-8", 422)
	}
	length := utf8.RuneCountInString(trimmed)
	if length < ManualEvidenceMinRunes || length > ManualEvidenceMaxRunes {
		return "", failure("invalid_evidence", "Ownership evidence must contain 20 to 4000 characters", 422)
	}
	return trimmed, nil
}

func validateAttachmentIDs(ids []string) error {
	if len(ids) > ManualAttachmentMaxCount {
		return failure("manual_attachment_invalid", "At most three evidence attachments are allowed", 422)
	}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			return failure("manual_attachment_invalid", "Evidence attachment identifiers are invalid", 422)
		}
		if _, exists := seen[id]; exists {
			return failure("manual_attachment_invalid", "Evidence attachments must be unique", 422)
		}
		seen[id] = struct{}{}
	}
	return nil
}
