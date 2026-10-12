package sitestats

import (
	"context"
	"fmt"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"io"
	"log/slog"
	"sync"
	"testing"
	"testing/synctest"
)

type graphMetricStore struct {
	Store
	mu      sync.Mutex
	counts  map[string]int64
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func TestGraphMetricsDrainAcceptedWritesAfterCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Given a write accepted before the reader disconnects.
		store := &graphMetricStore{counts: map[string]int64{}, started: make(chan struct{}), release: make(chan struct{})}
		service := &Service{store: store, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
		ctx, cancel := context.WithCancel(t.Context())
		returned, closed := make(chan struct{}), make(chan struct{})
		go func() { service.RecordGraph(ctx, []string{"A1b2C3d4E"}); close(returned) }()
		<-store.started
		// When cancellation and shutdown happen while persistence is blocked.
		cancel()
		go func() { service.Close(); close(closed) }()
		synctest.Wait()
		// Then both wait for the accepted event, and closing rejects later events.
		select {
		case <-closed:
			t.Fatal("shutdown abandoned accepted write")
		default:
		}
		select {
		case <-returned:
			t.Fatal("reader returned before accepted write")
		default:
		}
		close(store.release)
		<-closed
		<-returned
		service.RecordGraph(t.Context(), []string{"A1b2C3d4E"})
		if store.counts["A1b2C3d4E"] != 1 {
			t.Fatalf("count = %d", store.counts["A1b2C3d4E"])
		}
	})
}

func (store *graphMetricStore) IncrementGraphMetrics(_ context.Context, parameters dbgen.IncrementGraphMetricsParams) error {
	store.once.Do(func() { close(store.started) })
	<-store.release
	store.mu.Lock()
	defer store.mu.Unlock()
	for i, id := range parameters.ShortIds {
		store.counts[id] += parameters.Increments[i]
	}
	return nil
}
func TestGraphMetricsWaitForWritesAndCountEveryConcurrentDisplay(t *testing.T) {
	store := &graphMetricStore{counts: map[string]int64{}, started: make(chan struct{}), release: make(chan struct{})}
	service := &Service{store: store, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	returned := make(chan struct{}, 10)
	ids := make([]string, 9001)
	for i := range ids {
		ids[i] = fmt.Sprintf("%09d", i)
	}
	ids = append(ids, ids[0])
	var readers sync.WaitGroup
	for range 10 {
		readers.Go(func() {
			service.RecordGraph(t.Context(), ids)
			returned <- struct{}{}
		})
	}
	<-store.started
	select {
	case <-returned:
		t.Fatal("display returned before its write finished")
	default:
	}
	close(store.release)
	readers.Wait()
	if len(store.counts) != 9001 {
		t.Fatalf("counted %d distinct sites", len(store.counts))
	}
	for id, count := range store.counts {
		if count != 10 {
			t.Fatalf("site %s count=%d", id, count)
		}
	}
}
