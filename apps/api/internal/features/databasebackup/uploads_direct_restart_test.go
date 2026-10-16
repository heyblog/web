package databasebackup

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func restartDirect(t *testing.T, previous *Uploads) *Uploads {
	t.Helper()
	previous.Close()
	restarted, err := openUploads(t.Context(), previous.metadata, nil, "direct/", previous.directory, "", previous.logger)
	require.NoError(t, err)
	t.Cleanup(restarted.Close)
	return restarted
}

func TestDirectRestartReconcilesActualParts(t *testing.T) {
	// Given: two recorded parts, but one was corrupted while the API was stopped.
	uploads, _ := testDirectUploads(t)
	data := append(bytes.Repeat([]byte("a"), int(UploadPartSize)), 'z')
	session, err := uploads.Create(t.Context(), "actor", int64(len(data)), partHashes(data))
	require.NoError(t, err)
	require.NoError(t, receivePart(t.Context(), uploads, session.ID, 1, data[:UploadPartSize]))
	require.NoError(t, receivePart(t.Context(), uploads, session.ID, 2, data[UploadPartSize:]))
	uploads.Close()
	require.NoError(t, os.WriteFile(uploads.partPath(session.ID, 2), []byte("X"), 0600))
	// When
	restarted := restartDirect(t, uploads)
	// Then: only verified bytes count; the missing part can be replaced.
	current, err := restarted.Get(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	require.Equal(t, []int{1}, current.UploadedParts)
	require.Equal(t, UploadPartSize, current.UploadedBytes)
	require.NoError(t, receivePart(t.Context(), restarted, session.ID, 2, data[UploadPartSize:]))
	_, err = restarted.Complete(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	require.Equal(t, "ready", waitUpload(t, restarted, session.ID, "ready").State)
}

func TestDirectRestartPreservesVerifiedInspection(t *testing.T) {
	// Given
	uploads, _ := testDirectUploads(t)
	data := []byte("synthetic")
	session, err := uploads.Create(t.Context(), "actor", int64(len(data)), partHashes(data))
	require.NoError(t, err)
	require.NoError(t, receivePart(t.Context(), uploads, session.ID, 1, data))
	_, err = uploads.Complete(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	ready := waitUpload(t, uploads, session.ID, "ready")
	uploads.mu.Lock()
	uploads.entries[session.ID].record.State = "inspected"
	uploads.entries[session.ID].record.Inspection = &Inspection{SHA256: ready.SHA256, CanRestore: true}
	require.NoError(t, uploads.metadata.Save(t.Context(), uploads.entries[session.ID].record))
	uploads.mu.Unlock()
	// When
	restarted := restartDirect(t, uploads)
	// Then
	current, err := restarted.Get(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	require.Equal(t, "inspected", current.State)
	require.Equal(t, ready.SHA256, current.SHA256)
	require.NotNil(t, current.Inspection)
}

func TestDirectRestartRecoversPublishedFileAfterInterruptedPreparation(t *testing.T) {
	// Given: rename succeeded, but the process stopped before persisting readiness.
	uploads, _ := testDirectUploads(t)
	data := []byte("synthetic")
	session, err := uploads.Create(t.Context(), "actor", int64(len(data)), partHashes(data))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(uploads.localPath(session.ID), data, 0600))
	uploads.mu.Lock()
	uploads.entries[session.ID].record.State = "preparing"
	require.NoError(t, uploads.metadata.Save(t.Context(), uploads.entries[session.ID].record))
	uploads.mu.Unlock()
	// When
	restarted := restartDirect(t, uploads)
	// Then: readiness must allow automatic inspection, without a stale error.
	current, err := restarted.Get(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	require.Equal(t, "ready", current.State)
	require.Empty(t, current.ErrorCode)
	require.Equal(t, int64(len(data)), current.PreparedBytes)
}

func TestDirectRestartResetsLostDataAndClearsInspection(t *testing.T) {
	// Given
	uploads, _ := testDirectUploads(t)
	data := []byte("synthetic")
	session, err := uploads.Create(t.Context(), "actor", int64(len(data)), partHashes(data))
	require.NoError(t, err)
	require.NoError(t, receivePart(t.Context(), uploads, session.ID, 1, data))
	_, err = uploads.Complete(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	ready := waitUpload(t, uploads, session.ID, "ready")
	uploads.mu.Lock()
	uploads.entries[session.ID].record.State = "inspected"
	uploads.entries[session.ID].record.Inspection = &Inspection{SHA256: ready.SHA256, CanRestore: true}
	require.NoError(t, uploads.metadata.Save(t.Context(), uploads.entries[session.ID].record))
	uploads.mu.Unlock()
	uploads.Close()
	require.NoError(t, os.RemoveAll(uploads.directory))
	// When
	restarted := restartDirect(t, uploads)
	// Then
	current, err := restarted.Get(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	require.Equal(t, "uploading", current.State)
	require.Empty(t, current.SHA256)
	require.Nil(t, current.Inspection)
	require.Empty(t, current.UploadedParts)
	require.NoError(t, receivePart(t.Context(), restarted, session.ID, 1, data))
}

func TestDirectRestartDoesNotReplayRestore(t *testing.T) {
	// Given: persisted intent does not prove whether the database committed.
	uploads, _ := testDirectUploads(t)
	session, err := uploads.Create(t.Context(), "actor", 1, partHashes([]byte("x")))
	require.NoError(t, err)
	uploads.mu.Lock()
	record := &uploads.entries[session.ID].record
	record.State = "restoring"
	record.Job = &RestoreJob{ID: fixtureID, Status: "running"}
	require.NoError(t, uploads.metadata.Save(t.Context(), *record))
	uploads.mu.Unlock()
	// When
	restarted := restartDirect(t, uploads)
	// Then
	current, err := restarted.Get(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	require.Equal(t, "unknown", current.State)
	job, err := restarted.StoredJob("actor", fixtureID)
	require.NoError(t, err)
	require.Equal(t, "unknown", job.Status)
	_, err = restarted.Complete(t.Context(), "actor", session.ID)
	require.ErrorIs(t, err, ErrUploadState)
}

func TestUploadModeChangesRequireCleanup(t *testing.T) {
	// Given
	uploads, _ := testDirectUploads(t)
	session, err := uploads.Create(t.Context(), "actor", 1, partHashes([]byte("x")))
	require.NoError(t, err)
	uploads.Close()
	storage := &memoryUploadStorage{}
	// When / Then: unresolved direct resources prohibit switching to COS.
	_, err = openUploads(t.Context(), uploads.metadata, storage, "backups/", t.TempDir(), storage.Identity(), uploads.logger)
	require.ErrorIs(t, err, ErrUploadStorageChanged)
	uploads.mu.Lock()
	record := &uploads.entries[session.ID].record
	record.State, record.CleanupStatus, record.CompletedAt = "cancelled", "completed", time.Now()
	require.NoError(t, uploads.metadata.Save(t.Context(), *record))
	uploads.mu.Unlock()
	// A completed historical tombstone remains readable after switching modes.
	cos, err := openUploads(t.Context(), uploads.metadata, storage, "backups/", t.TempDir(), storage.Identity(), uploads.logger)
	require.NoError(t, err)
	t.Cleanup(cos.Close)
	history, err := cos.Get(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", history.State)
}
