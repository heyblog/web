package databasebackup

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type blockingInitiateStorage struct {
	*memoryUploadStorage
	started chan struct{}
}

func (store *blockingInitiateStorage) Initiate(ctx context.Context, _ string) (string, error) {
	close(store.started)
	<-ctx.Done()
	return "", ctx.Err()
}

func TestUploadCancelCanInterruptSlowInitiate(t *testing.T) {
	// Given
	uploads, storage, _ := testUploads(t, []byte("x"))
	blocking := &blockingInitiateStorage{storage, make(chan struct{})}
	uploads.mu.Lock()
	uploads.storage = blocking
	uploads.mu.Unlock()
	result := make(chan error, 1)
	go func() { _, err := uploads.Create(t.Context(), "actor", 1, partHashes([]byte("x"))); result <- err }()
	select {
	case <-blocking.started:
	case <-time.After(time.Second):
		t.Fatal("initiate never started")
	}
	session, err := uploads.Current(t.Context(), "actor")
	require.NoError(t, err)
	require.NotNil(t, session)
	// When
	_, err = uploads.Delete(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	// Then
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("initiate not cancelled")
	}
	cancelled, err := uploads.Get(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", cancelled.State)
}

func TestUploadShutdownJoinsSlowInitiate(t *testing.T) {
	// Given
	uploads, storage, _ := testUploads(t, []byte("x"))
	blocking := &blockingInitiateStorage{storage, make(chan struct{})}
	uploads.mu.Lock()
	uploads.storage = blocking
	uploads.mu.Unlock()
	result := make(chan error, 1)
	go func() { _, err := uploads.Create(t.Context(), "actor", 1, partHashes([]byte("x"))); result <- err }()
	select {
	case <-blocking.started:
	case <-time.After(time.Second):
		t.Fatal("initiate never started")
	}
	// When
	closed := make(chan struct{})
	go func() { uploads.Close(); close(closed) }()
	// Then
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("shutdown blocked on COS initiate")
	}
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("request not joined")
	}
}

type failPreparingMetadata struct {
	*memoryUploadMetadata
	fail atomic.Bool
}

func (store *failPreparingMetadata) Save(ctx context.Context, record uploadRecord) error {
	if record.State == "preparing" && store.fail.CompareAndSwap(true, false) {
		return errors.New("metadata unavailable")
	}
	return store.memoryUploadMetadata.Save(ctx, record)
}

func TestUploadCompleteDoesNotGetStuckAfterMetadataFailure(t *testing.T) {
	// Given
	uploads, _, metadata := testUploads(t, []byte("x"))
	failing := &failPreparingMetadata{memoryUploadMetadata: metadata}
	uploads.mu.Lock()
	uploads.metadata = failing
	uploads.mu.Unlock()
	session, err := uploads.Create(t.Context(), "actor", 1, partHashes([]byte("x")))
	require.NoError(t, err)
	failing.fail.Store(true)
	_, err = uploads.Complete(t.Context(), "actor", session.ID)
	require.Error(t, err)
	current, err := uploads.Get(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	require.Equal(t, "uploading", current.State)
	// When
	_, err = uploads.Complete(t.Context(), "actor", session.ID)
	require.NoError(t, err)
	// Then
	require.Equal(t, "ready", waitUpload(t, uploads, session.ID, "ready").State)
}
