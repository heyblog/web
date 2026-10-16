package databasebackup

import (
	"context"
	"errors"
	"io"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type ambiguousInitStorage struct {
	*memoryUploadStorage
	attempts int
}

func (store *ambiguousInitStorage) Initiate(ctx context.Context, key string) (string, error) {
	store.attempts++
	if store.attempts == 1 {
		return "", io.ErrUnexpectedEOF
	}
	return store.memoryUploadStorage.Initiate(ctx, key)
}

func TestUploadCleanupWaitsForEarlierAmbiguousInitiate(t *testing.T) {
	// Given: an initial create may still finish remotely after its response failed.
	uploads, storage, _ := testUploads(t, []byte("x"))
	uncertain := &ambiguousInitStorage{memoryUploadStorage: storage}
	var elapsed atomic.Int64
	initial := time.Now()
	uploads.mu.Lock()
	uploads.storage = uncertain
	uploads.now = func() time.Time { return initial.Add(time.Duration(elapsed.Load())) }
	uploads.mu.Unlock()
	_, err := uploads.Create(t.Context(), "actor", 1, partHashes([]byte("x")))
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	// When: an immediate retry creates a known upload, then the user cancels.
	session, err := uploads.Current(t.Context(), "actor")
	require.NoError(t, err)
	require.NotNil(t, session)
	_, err = uploads.Delete(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	uploads.sweepEntry(session.ID)
	// Then: retry success cannot erase the earlier unresolved remote operation.
	pending, err := uploads.Get(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	require.Equal(t, "pending", pending.CleanupStatus)
	storage.mu.Lock()
	require.Zero(t, storage.cleanups)
	storage.mu.Unlock()
	elapsed.Store(int64(Timeout + time.Second))
	uploads.sweepEntry(session.ID)
	require.Eventually(t, func() bool {
		completed, err := uploads.Get(t.Context(), "actor", session.ID)
		return err == nil && completed.CleanupStatus == "completed"
	}, time.Second, time.Millisecond)
}

type blockingDownloadStorage struct {
	*memoryUploadStorage
	started chan struct{}
	stopped chan struct{}
}
type cancelDownload struct {
	ctx     context.Context
	stopped chan struct{}
}

func (reader *cancelDownload) Read([]byte) (int, error) {
	<-reader.ctx.Done()
	return 0, reader.ctx.Err()
}
func (reader *cancelDownload) Close() error { close(reader.stopped); return nil }
func (store *blockingDownloadStorage) Open(ctx context.Context, _ string, _ remoteObject) (io.ReadCloser, error) {
	close(store.started)
	return &cancelDownload{ctx, store.stopped}, nil
}

func TestUploadCancelJoinsDownloadBeforeCleaning(t *testing.T) {
	// Given
	uploads, storage, _ := testUploads(t, []byte("x"))
	blocking := &blockingDownloadStorage{storage, make(chan struct{}), make(chan struct{})}
	uploads.mu.Lock()
	uploads.storage = blocking
	uploads.mu.Unlock()
	session, err := uploads.Create(t.Context(), "actor", 1, partHashes([]byte("x")))
	require.NoError(t, err)
	_, err = uploads.Complete(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	select {
	case <-blocking.started:
	case <-time.After(time.Second):
		t.Fatal("download never started")
	}
	// When
	_, err = uploads.Delete(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	// Then
	select {
	case <-blocking.stopped:
	case <-time.After(time.Second):
		t.Fatal("download not cancelled")
	}
	require.Eventually(t, func() bool {
		session, err := uploads.Get(t.Context(), "actor", session.ID)
		return err == nil && session.State == "cancelled" && session.CleanupStatus == "completed"
	}, time.Second, time.Millisecond)
	_, err = os.Stat(uploads.localPath(session.ID) + ".partial")
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestUploadCannotBeDeletedWhileInspectionUsesFile(t *testing.T) {
	// Given
	uploads := fixtureUploads(t)
	uploads.mu.Lock()
	uploads.entries[fixtureID].leased = true
	uploads.entries[fixtureID].record.State = "inspecting"
	uploads.mu.Unlock()
	// When
	_, err := uploads.Delete(t.Context(), "actor", fixtureID)
	// Then
	require.ErrorIs(t, err, ErrBusy)
	_, err = os.Stat(uploads.localPath(fixtureID))
	require.NoError(t, err)
	uploads.mu.Lock()
	uploads.entries[fixtureID].leased = false
	uploads.mu.Unlock()
}

func TestUploadDeleteKeepsTerminalOutcome(t *testing.T) {
	for _, state := range []string{"succeeded", "failed", "unknown"} {
		t.Run(state, func(t *testing.T) {
			// Given
			uploads := fixtureUploads(t)
			uploads.mu.Lock()
			uploads.entries[fixtureID].record.State = state
			uploads.mu.Unlock()
			// When
			session, err := uploads.Delete(t.Context(), "actor", fixtureID)
			// Then
			require.NoError(t, err)
			require.Equal(t, state, session.State)
			_, err = uploads.Delete(t.Context(), "actor", fixtureID)
			require.NoError(t, err)
		})
	}
}

func TestUploadRestartRejectsStorageConfigurationDrift(t *testing.T) {
	// Given
	uploads, storage, metadata := testUploads(t, []byte("x"))
	_, err := uploads.Create(t.Context(), "actor", 1, partHashes([]byte("x")))
	require.NoError(t, err)
	uploads.Close()
	storage.identity = "https://different.cos.ap-singapore.myqcloud.com"
	// When
	_, err = openUploads(t.Context(), metadata, storage, "backups/", t.TempDir(), storage.identity, uploads.logger)
	// Then
	require.ErrorIs(t, err, ErrUploadStorageChanged)
	storage.mu.Lock()
	require.Zero(t, storage.cleanups)
	storage.mu.Unlock()
}

func TestUploadRestartKeepsCleanedHistoryAfterStorageConfigurationChange(t *testing.T) {
	// Given: only a completed tombstone remains in the previous storage namespace.
	uploads := fixtureUploads(t)
	uploads.mu.Lock()
	entry := uploads.entries[fixtureID]
	entry.record.State, entry.record.CleanupStatus = "succeeded", "completed"
	entry.record.CompletedAt = time.Now()
	require.NoError(t, uploads.metadata.Save(t.Context(), entry.record))
	uploads.mu.Unlock()
	uploads.Close()
	storage := uploads.storage.(*memoryUploadStorage)
	storage.identity = "https://different.cos.ap-singapore.myqcloud.com"
	// When
	restarted, err := openUploads(t.Context(), uploads.metadata, storage, "new-backups/", t.TempDir(), storage.identity, uploads.logger)
	require.NoError(t, err)
	t.Cleanup(restarted.Close)
	// Then: the historical outcome is retained and new uploads use the new prefix.
	history, err := restarted.Get(t.Context(), "actor", fixtureID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", history.State)
	require.Equal(t, "completed", history.CleanupStatus)
	session, err := restarted.Create(t.Context(), "actor", 1, partHashes([]byte("x")))
	require.NoError(t, err)
	restarted.mu.Lock()
	key := restarted.entries[session.ID].record.Key
	restarted.mu.Unlock()
	require.Equal(t, "new-backups/"+session.ID+".json", key)
	storage.mu.Lock()
	cleanups := storage.cleanups
	storage.mu.Unlock()
	require.Zero(t, cleanups)
}

func TestUploadRestartRejectsPrefixChangeWhileResourcesRemain(t *testing.T) {
	// Given
	uploads, storage, metadata := testUploads(t, []byte("x"))
	_, err := uploads.Create(t.Context(), "actor", 1, partHashes([]byte("x")))
	require.NoError(t, err)
	uploads.Close()
	// When / Then
	_, err = openUploads(t.Context(), metadata, storage, "new-backups/", t.TempDir(), uploads.settings.UploadOrigin, uploads.logger)
	require.ErrorIs(t, err, ErrInvalid)
	storage.mu.Lock()
	cleanups := storage.cleanups
	storage.mu.Unlock()
	require.Zero(t, cleanups)
}

func TestUploadExpiryDoesNotDropPendingCleanupMetadata(t *testing.T) {
	// Given
	uploads, storage, metadata := testUploads(t, []byte("x"))
	session, err := uploads.Create(t.Context(), "actor", 1, partHashes([]byte("x")))
	require.NoError(t, err)
	storage.mu.Lock()
	storage.cleanupErr = errors.New("unavailable")
	storage.mu.Unlock()
	uploads.mu.Lock()
	uploads.entries[session.ID].record.ExpiresAt = time.Now().Add(-time.Hour)
	uploads.mu.Unlock()
	// When
	uploads.sweepEntry(session.ID)
	// Then
	expired, err := uploads.Get(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	require.Equal(t, "expired", expired.State)
	require.Equal(t, "pending", expired.CleanupStatus)
	records, err := metadata.Load(t.Context())
	require.NoError(t, err)
	require.Len(t, records, 1)
}
