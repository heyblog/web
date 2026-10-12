package publicview

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func (queryStub) GetPublicGraphRevision(context.Context) (int64, error) { return 0, nil }
func (queries graphQueriesStub) GetPublicGraphRevision(context.Context) (int64, error) {
	return 0, queries.err
}

func TestGraphWaitCanBeCancelledDuringAnotherLoad(t *testing.T) {
	// Given a cache load that owns the gate.
	service := &Service{graph: queryStub{}}
	service.graphOnce.Do(func() { service.graphGate = make(chan struct{}, 1) })
	service.graphGate <- struct{}{}
	ctx, cancel := context.WithCancel(t.Context())
	finished := make(chan error, 1)
	go func() { _, err := service.Graph(ctx); finished <- err }()
	// When the waiting reader disconnects.
	cancel()
	// Then it returns without waiting for the active load.
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("cancelled graph read succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled graph reader remained blocked")
	}
}

func TestGraphCacheHitCanReadWhileRefreshGateIsOccupied(t *testing.T) {
	// Given a committed snapshot and another reader holding the refresh gate.
	service := &Service{graph: queryStub{}}
	if _, err := service.Graph(t.Context()); err != nil {
		t.Fatal(err)
	}
	service.graphGate <- struct{}{}
	defer func() { <-service.graphGate }()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	// When the same committed generation is requested.
	_, err := service.Graph(ctx)
	// Then a cache hit completes without waiting for the refresh gate.
	if err != nil {
		t.Fatal(err)
	}
}

func (queryStub) GetPublicFriendGraph(context.Context, pgtype.UUID) ([]byte, error) {
	return []byte(`{"nodes":[],"edges":[],"stats":{"nodes":0,"edges":0,"reciprocalPairs":0},"centerId":null}`), nil
}

type graphQueriesStub struct {
	data []byte
	err  error
}

func (queries graphQueriesStub) GetPublicFriendGraph(context.Context, pgtype.UUID) ([]byte, error) {
	return queries.data, queries.err
}

func TestGraphPreservesExternalAndReciprocalData(t *testing.T) {
	// Given
	service := &Service{graph: graphQueriesStub{data: []byte(`{"nodes":[{"id":"external:example.com","name":"example.com","host":"example.com","homepageUrl":"https://example.com/blog","shortId":null,"customId":null}],"edges":[{"source":"site:123456789","target":"external:example.com","reciprocal":false}],"stats":{"nodes":1,"edges":1,"reciprocalPairs":0},"centerId":null}`)}}
	// When
	graph, err := service.Graph(context.Background())
	// Then
	if err != nil || len(graph.Nodes) != 1 || graph.Nodes[0].ShortID != nil || graph.Nodes[0].HomepageURL != "https://example.com/blog" || graph.Edges[0].Reciprocal {
		t.Fatalf("graph = %#v, error = %v", graph, err)
	}
}

func TestGraphRejectsMalformedSnapshot(t *testing.T) {
	// Given
	service := &Service{graph: graphQueriesStub{data: []byte(`{`)}}
	// When
	_, err := service.Graph(context.Background())
	// Then
	if err == nil {
		t.Fatal("malformed graph accepted")
	}
}

type cachedGraphQueries struct {
	graphQueriesStub
	revision int64
	calls    atomic.Int64
}

func (queries *cachedGraphQueries) GetPublicGraphRevision(context.Context) (int64, error) {
	return queries.revision, nil
}
func (queries *cachedGraphQueries) GetPublicFriendGraph(ctx context.Context, center pgtype.UUID) ([]byte, error) {
	queries.calls.Add(1)
	return queries.graphQueriesStub.GetPublicFriendGraph(ctx, center)
}
func TestGlobalGraphCoalescesReadsAndReloadsCommittedGenerations(t *testing.T) {
	queries := &cachedGraphQueries{graphQueriesStub: graphQueriesStub{data: []byte(`{"nodes":[],"edges":[],"stats":{"nodes":0}}`)}}
	service := &Service{graph: queries}
	var readers sync.WaitGroup
	for range 10 {
		readers.Go(func() {
			if _, err := service.Graph(t.Context()); err != nil {
				t.Error(err)
			}
		})
	}
	readers.Wait()
	if queries.calls.Load() != 1 {
		t.Fatalf("snapshot reads=%d", queries.calls.Load())
	}
	queries.revision++
	queries.data = []byte(`{`)
	if _, err := service.Graph(t.Context()); err == nil {
		t.Fatal("failed refresh served stale data")
	}
	queries.data = []byte(`{"nodes":[],"edges":[],"stats":{"nodes":2}}`)
	graph, err := service.Graph(t.Context())
	if err != nil || graph.Stats.Nodes != 2 || queries.calls.Load() != 3 {
		t.Fatalf("refresh=%#v err=%v reads=%d", graph, err, queries.calls.Load())
	}
}
