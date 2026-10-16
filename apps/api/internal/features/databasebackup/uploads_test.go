package databasebackup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const fixtureID = "00000000-0000-4000-8000-000000000001"
const fixtureChecksum = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type memoryUploadMetadata struct {
	mu      sync.Mutex
	records map[string][]byte
}

func (store *memoryUploadMetadata) Load(context.Context) ([]uploadRecord, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	var result []uploadRecord
	for _, data := range store.records {
		var record uploadRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	return result, nil
}
func (store *memoryUploadMetadata) Save(_ context.Context, record uploadRecord) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	store.records[record.ID] = data
	return nil
}
func (store *memoryUploadMetadata) Delete(_ context.Context, id string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	delete(store.records, id)
	return nil
}

type memoryUploadStorage struct {
	mu           sync.Mutex
	data         []byte
	initiated    bool
	completed    bool
	cleanupErr   error
	cleanups     int
	completeLost bool
	identity     string
}

func (store *memoryUploadStorage) Identity() string {
	if store.identity != "" {
		return store.identity
	}
	return "https://synthetic.cos.ap-singapore.myqcloud.com"
}

func (store *memoryUploadStorage) Initiate(context.Context, string) (string, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.initiated = true
	return "multipart", nil
}
func (store *memoryUploadStorage) Uploads(context.Context, string) ([]string, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.initiated && !store.completed {
		return []string{"multipart"}, nil
	}
	return nil, nil
}
func (store *memoryUploadStorage) Parts(context.Context, string, string) ([]remotePart, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	parts := []remotePart{}
	for offset := int64(0); offset < int64(len(store.data)); offset += UploadPartSize {
		parts = append(parts, remotePart{len(parts) + 1, min(UploadPartSize, int64(len(store.data))-offset), "part-etag"})
	}
	return parts, nil
}
func (store *memoryUploadStorage) Sign(context.Context, string, string, int, int64) (string, error) {
	return "https://synthetic.cos.ap-singapore.myqcloud.com/part", nil
}
func (store *memoryUploadStorage) Complete(context.Context, string, string, []remotePart) (remoteObject, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.completed = true
	if store.completeLost {
		return remoteObject{}, io.ErrUnexpectedEOF
	}
	return remoteObject{"object-etag", "version-1", int64(len(store.data))}, nil
}
func (store *memoryUploadStorage) Head(context.Context, string) (remoteObject, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.completed {
		return remoteObject{}, errRemoteMissing
	}
	return remoteObject{"object-etag", "version-1", int64(len(store.data))}, nil
}
func (store *memoryUploadStorage) Open(_ context.Context, _ string, object remoteObject) (io.ReadCloser, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if object.ETag != "object-etag" || object.Version != "version-1" {
		return nil, ErrInvalid
	}
	return io.NopCloser(bytes.NewReader(bytes.Clone(store.data))), nil
}
func (store *memoryUploadStorage) Cleanup(context.Context, string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.cleanups++
	if store.cleanupErr != nil {
		return store.cleanupErr
	}
	store.data = nil
	store.initiated = false
	store.completed = false
	return nil
}

func testUploads(t *testing.T, data []byte) (*Uploads, *memoryUploadStorage, *memoryUploadMetadata) {
	t.Helper()
	storage := &memoryUploadStorage{data: bytes.Clone(data)}
	metadata := &memoryUploadMetadata{records: map[string][]byte{}}
	uploads, err := openUploads(t.Context(), metadata, storage, "backups/", t.TempDir(), "https://synthetic.cos.ap-singapore.myqcloud.com", slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	t.Cleanup(uploads.Close)
	return uploads, storage, metadata
}

func fixtureUploads(t *testing.T) *Uploads {
	t.Helper()
	uploads, _, metadata := testUploads(t, []byte(`{"synthetic":true}`))
	now := time.Now()
	record := uploadRecord{UploadSession: UploadSession{ID: fixtureID, Size: 18, PartSize: UploadPartSize, PartSHA256: []string{fixtureChecksum}, UploadedParts: []int{1}, State: "ready", ExpiresAt: now.Add(time.Hour), SHA256: fixtureChecksum, CleanupStatus: "not_required"}, Owner: "actor", Key: "backups/" + fixtureID + ".json", CreatedAt: now}
	record.StorageIdentity = uploads.storage.Identity()
	require.NoError(t, os.WriteFile(uploads.localPath(fixtureID), []byte(`{"synthetic":true}`), 0600))
	require.NoError(t, metadata.Save(t.Context(), record))
	uploads.mu.Lock()
	uploads.entries[fixtureID] = &uploadEntry{record: record}
	uploads.mu.Unlock()
	return uploads
}

func partHashes(data []byte) []string {
	hashes := []string{}
	for offset := int64(0); offset < int64(len(data)); offset += UploadPartSize {
		digest := sha256.Sum256(data[offset:min(offset+UploadPartSize, int64(len(data)))])
		hashes = append(hashes, hex.EncodeToString(digest[:]))
	}
	return hashes
}

func waitUpload(t *testing.T, uploads *Uploads, id, state string) UploadSession {
	t.Helper()
	var session UploadSession
	require.Eventually(t, func() bool {
		var err error
		session, err = uploads.Get(t.Context(), "actor", id)
		return err == nil && session.State == state
	}, 3*time.Second, time.Millisecond)
	return session
}

func TestUploadPreparesOnlyVerifiedCompleteFile(t *testing.T) {
	// Given
	data := bytes.Repeat([]byte("a"), int(UploadPartSize)+17)
	uploads, _, _ := testUploads(t, data)
	session, err := uploads.Create(t.Context(), "actor", int64(len(data)), partHashes(data))
	require.NoError(t, err)
	// When
	_, err = uploads.Complete(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	ready := waitUpload(t, uploads, session.ID, "ready")
	// Then
	actual, err := os.ReadFile(uploads.localPath(session.ID))
	require.NoError(t, err)
	require.Equal(t, data, actual)
	digest := sha256.Sum256(data)
	require.Equal(t, hex.EncodeToString(digest[:]), ready.SHA256)
	require.Equal(t, int64(len(data)), ready.PreparedBytes)
	_, err = os.Stat(uploads.localPath(session.ID) + ".partial")
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestUploadRejectsDifferentContentBeforeLocalReady(t *testing.T) {
	// Given
	uploads, _, _ := testUploads(t, []byte("different"))
	session, err := uploads.Create(t.Context(), "actor", 9, partHashes([]byte("expected!")))
	require.NoError(t, err)
	// When
	_, err = uploads.Complete(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	failed := waitUpload(t, uploads, session.ID, "failed")
	// Then
	require.Equal(t, "database_backup_invalid", failed.ErrorCode)
	require.Contains(t, []string{"pending", "completed"}, failed.CleanupStatus)
	_, err = os.Stat(uploads.localPath(session.ID))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestUploadReconcilesLostCompleteResponse(t *testing.T) {
	// Given
	data := []byte("synthetic")
	uploads, storage, _ := testUploads(t, data)
	storage.completeLost = true
	session, err := uploads.Create(t.Context(), "actor", int64(len(data)), partHashes(data))
	require.NoError(t, err)
	// When
	_, err = uploads.Complete(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	// Then
	require.Equal(t, "ready", waitUpload(t, uploads, session.ID, "ready").State)
}

func TestUploadOwnerAndManifestAreEnforced(t *testing.T) {
	// Given
	uploads, _, _ := testUploads(t, []byte("x"))
	session, err := uploads.Create(t.Context(), "actor", 1, partHashes([]byte("x")))
	require.NoError(t, err)
	// When / Then
	_, err = uploads.Get(t.Context(), "other", session.ID)
	require.ErrorIs(t, err, ErrUploadNotFound)
	_, _, err = uploads.Sign(t.Context(), "actor", session.ID, 2)
	require.ErrorIs(t, err, ErrUploadState)
	_, err = uploads.Create(t.Context(), "actor", 1, partHashes([]byte("x")))
	require.ErrorIs(t, err, ErrBusy)
}

func TestUploadCleanupRetriesWithoutChangingRestoreOutcome(t *testing.T) {
	// Given
	uploads, storage, _ := testUploads(t, []byte("x"))
	session, err := uploads.Create(t.Context(), "actor", 1, partHashes([]byte("x")))
	require.NoError(t, err)
	uploads.mu.Lock()
	entry := uploads.entries[session.ID]
	entry.record.State = "succeeded"
	entry.record.CleanupStatus = "pending"
	uploads.mu.Unlock()
	storage.mu.Lock()
	storage.cleanupErr = errors.New("transient")
	storage.mu.Unlock()
	uploads.sweepEntry(session.ID)
	pending, err := uploads.Get(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	require.Equal(t, "pending", pending.CleanupStatus)
	// When
	storage.mu.Lock()
	storage.cleanupErr = nil
	storage.mu.Unlock()
	uploads.sweepEntry(session.ID)
	uploads.wakeCleanup()
	// Then
	var completed UploadSession
	require.Eventually(t, func() bool {
		completed, err = uploads.Get(t.Context(), "actor", session.ID)
		return err == nil && completed.CleanupStatus == "completed"
	}, time.Second, time.Millisecond)
	require.Equal(t, "succeeded", completed.State)
}

func TestUploadRestartRedownloadsMissingCache(t *testing.T) {
	// Given
	data := []byte("synthetic")
	uploads, storage, metadata := testUploads(t, data)
	session, err := uploads.Create(t.Context(), "actor", int64(len(data)), partHashes(data))
	require.NoError(t, err)
	_, err = uploads.Complete(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	waitUpload(t, uploads, session.ID, "ready")
	uploads.Close()
	// When
	restarted, err := openUploads(t.Context(), metadata, storage, "backups/", filepath.Join(t.TempDir(), "staging"), uploads.settings.UploadOrigin, uploads.logger)
	require.NoError(t, err)
	t.Cleanup(restarted.Close)
	// Then
	require.Equal(t, "ready", waitUpload(t, restarted, session.ID, "ready").State)
}
