package databasebackup

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDirectUploadCancellationJoinsBothWriters(t *testing.T) {
	// Given: both upload slots are occupied by paused streams.
	uploads, _ := testDirectUploads(t)
	session, err := uploads.Create(t.Context(), "actor", UploadPartSize+1, []string{fixtureChecksum, fixtureChecksum})
	require.NoError(t, err)
	results := make(chan error, 2)
	for number := 1; number <= 2; number++ {
		reader, writer := io.Pipe()
		t.Cleanup(func() { require.NoError(t, writer.Close()) })
		go func() {
			results <- uploads.ReceivePart(t.Context(), UploadPart{Owner: "actor", ID: session.ID, Number: number, Body: reader})
		}()
	}
	require.Eventually(t, func() bool {
		uploads.mu.Lock()
		defer uploads.mu.Unlock()
		return len(uploads.entries[session.ID].writers) == 2
	}, time.Second, time.Millisecond)
	require.ErrorIs(t, receivePart(t.Context(), uploads, session.ID, 1, []byte("x")), ErrBusy)
	_, err = uploads.Complete(t.Context(), "actor", session.ID)
	require.ErrorIs(t, err, ErrBusy)
	// When
	_, err = uploads.Delete(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	// Then
	for range 2 {
		select {
		case err := <-results:
			require.Error(t, err)
		case <-time.After(time.Second):
			t.Fatal("part writer was not joined")
		}
	}
	require.Eventually(t, func() bool {
		current, err := uploads.Get(t.Context(), "actor", session.ID)
		return err == nil && current.CleanupStatus == "completed"
	}, time.Second, time.Millisecond)
	files, err := os.ReadDir(uploads.directory)
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestDirectExpiryCleansPartsAndAssembledFile(t *testing.T) {
	// Given
	uploads, _ := testDirectUploads(t)
	session, err := uploads.Create(t.Context(), "actor", 1, partHashes([]byte("x")))
	require.NoError(t, err)
	require.NoError(t, receivePart(t.Context(), uploads, session.ID, 1, []byte("x")))
	_, err = uploads.Complete(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	waitUpload(t, uploads, session.ID, "ready")
	uploads.mu.Lock()
	uploads.entries[session.ID].record.ExpiresAt = time.Now().Add(-time.Second)
	uploads.mu.Unlock()
	// When
	uploads.sweepEntry(session.ID)
	// Then
	current, err := uploads.Get(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	require.Equal(t, "expired", current.State)
	require.Equal(t, "completed", current.CleanupStatus)
	files, err := os.ReadDir(uploads.directory)
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestDirectShutdownDiscardsInterruptedWrite(t *testing.T) {
	// Given
	uploads, _ := testDirectUploads(t)
	session, err := uploads.Create(t.Context(), "actor", UploadPartSize+1, []string{fixtureChecksum, fixtureChecksum})
	require.NoError(t, err)
	reader, writer := io.Pipe()
	defer func() { require.NoError(t, writer.Close()) }()
	result := make(chan error, 1)
	go func() {
		result <- uploads.ReceivePart(t.Context(), UploadPart{Owner: "actor", ID: session.ID, Number: 1, Body: reader})
	}()
	require.Eventually(t, func() bool {
		uploads.mu.Lock()
		defer uploads.mu.Unlock()
		return len(uploads.entries[session.ID].writers) == 1
	}, time.Second, time.Millisecond)
	// When
	uploads.Close()
	// Then
	select {
	case err := <-result:
		require.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("shutdown did not join writer")
	}
	restarted := restartDirect(t, uploads)
	current, err := restarted.Get(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	require.Equal(t, "uploading", current.State)
	require.Empty(t, current.UploadedParts)
	_, err = os.Stat(uploads.partPath(session.ID, 1))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestDirectResumeRejectsCancelledContext(t *testing.T) {
	// Given
	uploads, _ := testDirectUploads(t)
	session, err := uploads.Create(t.Context(), "actor", 1, partHashes([]byte("x")))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	// When
	err = receivePart(ctx, uploads, session.ID, 1, []byte("x"))
	// Then
	require.ErrorIs(t, err, context.Canceled)
	_, err = os.Stat(uploads.partPath(session.ID, 1))
	require.ErrorIs(t, err, os.ErrNotExist)
}
