package publicview

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"heyblog-api/internal/platform/apperror"
)

type GraphNode struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Host        string  `json:"host"`
	HomepageURL string  `json:"homepageUrl"`
	ShortID     *string `json:"shortId"`
	CustomID    *string `json:"customId"`
}

type GraphEdge struct {
	Source     string `json:"source"`
	Target     string `json:"target"`
	Reciprocal bool   `json:"reciprocal"`
}

type GraphStats struct {
	Nodes           int `json:"nodes"`
	Edges           int `json:"edges"`
	ReciprocalPairs int `json:"reciprocalPairs"`
}

type FriendGraph struct {
	encoded  []byte
	Nodes    []GraphNode `json:"nodes"`
	Edges    []GraphEdge `json:"edges"`
	Stats    GraphStats  `json:"stats"`
	CenterID *string     `json:"centerId"`
}

type cachedGraph struct {
	revision int64
	view     FriendGraph
}

func (service *Service) Graph(ctx context.Context) (FriendGraph, error) {
	graph, err := service.globalGraph(ctx)
	if err != nil {
		return FriendGraph{}, err
	}
	service.recordGraph(ctx, graph)
	return graph, nil
}

func (service *Service) globalGraph(ctx context.Context) (FriendGraph, error) {
	service.graphOnce.Do(func() { service.graphGate = make(chan struct{}, 1) })
	select {
	case service.graphGate <- struct{}{}:
		defer func() { <-service.graphGate }()
	case <-ctx.Done():
		return FriendGraph{}, internalError(ctx.Err(), "load friend graph")
	}
	// Read the committed generation before loading a snapshot. A concurrent commit may
	// make this result newer than the key, but the next request will then reload it.
	revision, err := service.graph.GetPublicGraphRevision(ctx)
	if err != nil {
		return FriendGraph{}, internalError(err, "read graph generation")
	}
	if err := ctx.Err(); err != nil {
		return FriendGraph{}, internalError(err, "load friend graph")
	}
	if service.graphSnapshot != nil && service.graphSnapshot.revision == revision {
		return service.graphSnapshot.view, nil
	}
	graph, err := service.decodeGraph(ctx, pgtype.UUID{})
	if err != nil {
		return FriendGraph{}, err
	}
	graph.encoded, err = json.Marshal(graph)
	if err != nil {
		return FriendGraph{}, internalError(err, "encode friend graph")
	}
	service.graphSnapshot = &cachedGraph{revision: revision, view: graph}
	return graph, nil
}

func (service *Service) SiteGraphByIdentifier(ctx context.Context, identifier SiteIdentifier) (FriendGraph, error) {
	row, err := siteRowByIdentifier(ctx, service.lookup, identifier)
	var applicationError *apperror.Error
	if errors.As(err, &applicationError) {
		return FriendGraph{}, err
	}
	if errors.Is(err, pgx.ErrNoRows) || err == nil && row.Visibility == "REMOVED" {
		return FriendGraph{}, notFound()
	}
	if err != nil {
		return FriendGraph{}, internalError(err, "resolve graph center")
	}
	return service.loadGraph(ctx, row.ID)
}

func (service *Service) loadGraph(ctx context.Context, centerID pgtype.UUID) (FriendGraph, error) {
	graph, err := service.decodeGraph(ctx, centerID)
	if err != nil {
		return FriendGraph{}, err
	}
	service.recordGraph(ctx, graph)
	return graph, nil
}

func (service *Service) decodeGraph(ctx context.Context, centerID pgtype.UUID) (FriendGraph, error) {
	data, err := service.graph.GetPublicFriendGraph(ctx, centerID)
	if err != nil {
		return FriendGraph{}, internalError(err, "load friend graph")
	}
	if len(data) == 0 || string(data) == "null" {
		return FriendGraph{}, notFound()
	}
	var graph FriendGraph
	if err := json.Unmarshal(data, &graph); err != nil {
		return FriendGraph{}, internalError(err, "decode friend graph")
	}
	return graph, nil
}

func (service *Service) recordGraph(ctx context.Context, graph FriendGraph) {
	if service.metrics != nil {
		ids := make([]string, 0, len(graph.Nodes))
		for _, node := range graph.Nodes {
			if node.ShortID != nil {
				ids = append(ids, *node.ShortID)
			}
		}
		service.metrics.RecordGraph(ctx, ids)
	}
}
