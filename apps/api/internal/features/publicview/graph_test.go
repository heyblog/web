package publicview

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

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
