package sitemanagement

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"hash/crc64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"heyblog-api/internal/platform/config"

	"github.com/stretchr/testify/require"

	cos "github.com/tencentyun/cos-go-sdk-v5"
)

func TestEvidenceStorage_promotesValidatedBytesToPrivateKey(t *testing.T) {
	// Given a temporary PDF and a separate final object.
	content := []byte("%PDF-1.7\nproof")
	digest := sha256.Sum256(content)
	id := "00000000-0000-4000-8000-000000000001"
	var saved []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/"+evidenceStoragePrefix+id+"-upload" {
			w.Header().Set("Content-Length", strconv.Itoa(len(content)))
			w.Header().Set("Content-Type", "application/pdf")
			if r.Method == http.MethodGet {
				_, _ = w.Write(content)
			}
			return
		}
		if r.URL.Path == "/"+evidenceStoragePrefix+id && r.Method == http.MethodPut {
			require.Equal(t, "private", r.Header.Get("x-cos-acl"))
			var err error
			saved, err = io.ReadAll(r.Body)
			require.NoError(t, err)
			w.Header().Set("x-cos-hash-crc64ecma", strconv.FormatUint(crc64.Checksum(saved, crc64.MakeTable(crc64.ECMA)), 10))
			return
		}
		t.Errorf("unexpected object request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	endpoint, err := url.Parse(server.URL)
	require.NoError(t, err)
	storage := &EvidenceStorage{enabled: true, prefix: evidenceStoragePrefix, client: cos.NewClient(&cos.BaseURL{BucketURL: endpoint}, server.Client())}
	// When completion validates the upload and the uploader subsequently changes it.
	object, err := storage.Complete(t.Context(), id, AttachmentUploadInput{Filename: "证明.pdf", ContentType: "application/pdf", Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:])})
	require.NoError(t, err)
	content[10] = 'X'
	// Then the final bytes remain the validated content, with the digest bound to it.
	require.NotEqual(t, content, saved)
	require.Equal(t, evidenceStoragePrefix+id, object.Key)
	require.Equal(t, hex.EncodeToString(digest[:]), object.SHA256)
}

type deniedEvidenceMetadata struct{}

func (deniedEvidenceMetadata) AttachmentMetadata(context.Context, string) (ClaimAttachment, error) {
	return ClaimAttachment{}, ErrNotFound
}
func (deniedEvidenceMetadata) CreateAttachmentMetadata(context.Context, string, Site, AttachmentUploadInput) (ClaimAttachment, error) {
	return ClaimAttachment{}, ErrNotFound
}
func (deniedEvidenceMetadata) CompleteAttachmentMetadata(context.Context, string, string, func(ClaimAttachment) (EvidenceObject, error)) (ClaimAttachment, error) {
	return ClaimAttachment{}, ErrNotFound
}
func (deniedEvidenceMetadata) DeleteAttachmentMetadata(context.Context, string, string) error {
	return ErrNotFound
}

func TestAttachmentCompletion_deniesForeignMetadataBeforeStorageAccess(t *testing.T) {
	// Given an attachment not accessible to this applicant, and no storage client.
	manager := &AttachmentManager{metadata: deniedEvidenceMetadata{}}
	// When completion is attempted, authorization runs before any object read.
	_, err := manager.CompleteAttachment(t.Context(), "another-user", "attachment", AttachmentUploadInput{})
	require.ErrorIs(t, err, ErrNotFound)
}

func TestEvidenceStorage_rejectsKeysOutsideAttachmentNamespace(t *testing.T) {
	storage := &EvidenceStorage{enabled: true, prefix: evidenceStoragePrefix}
	for _, key := range []string{"backup/object", evidenceStoragePrefix + "../object", evidenceStoragePrefix + "00000000-0000-4000-8000-000000000001/other"} {
		require.Error(t, storage.CleanupKey(t.Context(), key))
	}
}

type evidenceMetadataFixture struct {
	deniedEvidenceMetadata
	attachment ClaimAttachment
}

func (fixture evidenceMetadataFixture) AttachmentMetadata(context.Context, string) (ClaimAttachment, error) {
	return fixture.attachment, nil
}

func TestEvidenceDownload_requiresOwnershipOrReviewerAndRetainedBinding(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000001"
	storage, err := NewEvidenceStorage(config.BackupCOSConfig{Enabled: true, Bucket: "test-bucket", Region: "test-region", SecretID: "test-id", SecretKey: "test-key"})
	require.NoError(t, err)
	attachment := ClaimAttachment{ID: id, UserID: "owner", ClaimID: "claim", Filename: "proof.pdf", ContentType: AttachmentPDF, Status: AttachmentBound, StorageKey: evidenceStoragePrefix + id, StorageIdentity: storage.origin}
	manager := &AttachmentManager{storage: storage, metadata: evidenceMetadataFixture{attachment: attachment}}
	_, err = manager.DownloadAttachment(t.Context(), "another-account", id, false)
	require.ErrorIs(t, err, ErrNotFound)
	for _, access := range []struct {
		user     string
		reviewer bool
	}{{"owner", false}, {"reviewer", true}} {
		result, err := manager.DownloadAttachment(t.Context(), access.user, id, access.reviewer)
		require.NoError(t, err)
		parsed, err := url.Parse(result.URL)
		require.NoError(t, err)
		require.Equal(t, "/"+attachment.StorageKey, parsed.Path)
		require.Contains(t, parsed.Query().Get("response-content-disposition"), "attachment")
		require.Equal(t, "private, no-store", parsed.Query().Get("response-cache-control"))
	}
	for _, status := range []AttachmentStatus{AttachmentUploading, AttachmentReady, AttachmentCleanupPending, AttachmentDeleted} {
		attachment.Status = status
		manager.metadata = evidenceMetadataFixture{attachment: attachment}
		_, err := manager.DownloadAttachment(t.Context(), "reviewer", id, true)
		require.ErrorIs(t, err, ErrNotFound)
	}
	attachment.Status = AttachmentBound
	attachment.StorageIdentity = "https://another.cos.region.myqcloud.com"
	manager.metadata = evidenceMetadataFixture{attachment: attachment}
	_, err = manager.DownloadAttachment(t.Context(), "owner", id, false)
	require.Error(t, err)
}
