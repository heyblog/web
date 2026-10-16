package databasebackup

import (
	"context"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type liveContextMetadata struct{ *memoryUploadMetadata }

func (store liveContextMetadata) Save(ctx context.Context, record uploadRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return store.memoryUploadMetadata.Save(ctx, record)
}

type delayedInitiateStorage struct {
	*memoryUploadStorage
	fail     func()
	attempts int
}

func (store *delayedInitiateStorage) Initiate(ctx context.Context, key string) (string, error) {
	store.attempts++
	if store.attempts == 1 {
		store.fail()
		return "", ctx.Err()
	}
	return store.memoryUploadStorage.Initiate(ctx, key)
}

func TestUploadDelayedInitiatePersistsGraceAfterRequestCancellation(t *testing.T) {
	// Given: the ambiguous request outlasts its original grace and loses its context.
	uploads, storage, metadata := testUploads(t, []byte("x"))
	initial := time.Now().UTC()
	var elapsed atomic.Int64
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	uploads.mu.Lock()
	uploads.now = func() time.Time { return initial.Add(time.Duration(elapsed.Load())) }
	uploads.metadata = liveContextMetadata{metadata}
	uploads.storage = &delayedInitiateStorage{memoryUploadStorage: storage, fail: func() {
		elapsed.Store(int64(Timeout + time.Second))
		cancel()
	}}
	uploads.mu.Unlock()
	_, err := uploads.Create(ctx, "actor", 1, partHashes([]byte("x")))
	require.ErrorIs(t, err, context.Canceled)
	expected := initial.Add(2*Timeout + time.Second)
	records, err := metadata.Load(t.Context())
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, expected, records[0].RemoteUncertainUntil)

	// When: retry succeeds, its known UploadId cannot resolve the earlier request.
	session, err := uploads.Current(t.Context(), "actor")
	require.NoError(t, err)
	require.NotNil(t, session)
	records, err = metadata.Load(t.Context())
	require.NoError(t, err)
	require.Equal(t, expected, records[0].RemoteUncertainUntil)
	assertCleanupWaitsForDeadline(t, uploads, storage, session.ID, &elapsed, 2*Timeout+time.Second)
}

type delayedCompleteStorage struct {
	*memoryUploadStorage
	fail     func(context.Context) error
	onHead   func()
	attempts atomic.Int32
}

func (store *delayedCompleteStorage) Complete(ctx context.Context, key, id string, parts []remotePart) (remoteObject, error) {
	object, err := store.memoryUploadStorage.Complete(ctx, key, id, parts)
	if err != nil {
		return object, err
	}
	if store.attempts.Add(1) == 1 {
		return remoteObject{}, store.fail(ctx)
	}
	return object, nil
}

func (store *delayedCompleteStorage) Head(ctx context.Context, key string) (remoteObject, error) {
	if store.attempts.Load() > 0 {
		store.onHead()
	}
	if err := ctx.Err(); err != nil {
		return remoteObject{}, err
	}
	return store.memoryUploadStorage.Head(ctx, key)
}

func TestUploadDelayedCompleteKeepsGraceAcrossHeadAndRetry(t *testing.T) {
	for _, cancelPreparation := range []bool{false, true} {
		name := "head_reconciles"
		if cancelPreparation {
			name = "cancelled_head_then_retry"
		}
		t.Run(name, func(t *testing.T) {
			// Given: COS completes remotely after the original grace has elapsed.
			uploads, storage, metadata := testUploads(t, []byte("x"))
			initial := time.Now().UTC()
			var elapsed atomic.Int64
			graceAtHead := make(chan time.Time, 2)
			uploads.mu.Lock()
			uploads.now = func() time.Time { return initial.Add(time.Duration(elapsed.Load())) }
			uploads.metadata = liveContextMetadata{metadata}
			uploads.mu.Unlock()
			session, err := uploads.Create(t.Context(), "actor", 1, partHashes([]byte("x")))
			require.NoError(t, err)
			uploads.mu.Lock()
			uploads.storage = &delayedCompleteStorage{memoryUploadStorage: storage, fail: func(ctx context.Context) error {
				elapsed.Store(int64(Timeout + time.Second))
				if cancelPreparation {
					uploads.mu.Lock()
					uploads.entries[session.ID].cancel()
					uploads.mu.Unlock()
					return ctx.Err()
				}
				return io.ErrUnexpectedEOF
			}, onHead: func() {
				records, err := metadata.Load(context.Background())
				if err == nil && len(records) == 1 {
					graceAtHead <- records[0].RemoteUncertainUntil
				}
			}}
			uploads.mu.Unlock()
			_, err = uploads.Complete(t.Context(), "actor", session.ID)
			require.NoError(t, err)
			state := "ready"
			if cancelPreparation {
				state = "preparation_failed"
			}
			waitUpload(t, uploads, session.ID, state)
			expected := initial.Add(2*Timeout + time.Second)
			select {
			case persisted := <-graceAtHead:
				require.Equal(t, expected, persisted)
			default:
				t.Fatal("recovery Head did not observe persisted uncertainty")
			}
			records, err := metadata.Load(t.Context())
			require.NoError(t, err)
			require.Len(t, records, 1)
			require.Equal(t, expected, records[0].RemoteUncertainUntil)

			// When: a successful Head, now or on retry, observes the completed object.
			if cancelPreparation {
				_, err = uploads.Complete(t.Context(), "actor", session.ID)
				require.NoError(t, err)
				waitUpload(t, uploads, session.ID, "ready")
			}
			records, err = metadata.Load(t.Context())
			require.NoError(t, err)
			require.Equal(t, expected, records[0].RemoteUncertainUntil)
			assertCleanupWaitsForDeadline(t, uploads, storage, session.ID, &elapsed, 2*Timeout+time.Second)
		})
	}
}

func assertCleanupWaitsForDeadline(t *testing.T, uploads *Uploads, storage *memoryUploadStorage, id string, elapsed *atomic.Int64, deadline time.Duration) {
	t.Helper()
	_, err := uploads.Delete(t.Context(), "actor", id)
	require.NoError(t, err)
	elapsed.Store(int64(deadline - time.Second))
	uploads.sweepEntry(id)
	pending, err := uploads.Get(t.Context(), "actor", id)
	require.NoError(t, err)
	require.Equal(t, "pending", pending.CleanupStatus)
	storage.mu.Lock()
	cleanups := storage.cleanups
	storage.mu.Unlock()
	require.Zero(t, cleanups)
	elapsed.Store(int64(deadline + time.Second))
	uploads.wakeCleanup()
	require.Eventually(t, func() bool {
		completed, err := uploads.Get(t.Context(), "actor", id)
		return err == nil && completed.CleanupStatus == "completed"
	}, time.Second, time.Millisecond)
}
