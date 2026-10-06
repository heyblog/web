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
	Nodes    []GraphNode `json:"nodes"`
	Edges    []GraphEdge `json:"edges"`
	Stats    GraphStats  `json:"stats"`
	CenterID *string     `json:"centerId"`
}

func (service *Service) Graph(ctx context.Context) (FriendGraph, error) {
	return service.loadGraph(ctx, pgtype.UUID{})
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
	if service.metrics != nil {
		ids := make([]string, 0, len(graph.Nodes))
		for _, node := range graph.Nodes {
			if node.ShortID != nil {
				ids = append(ids, *node.ShortID)
			}
		}
		service.metrics.RecordQuery(ctx, ids)
		service.metrics.RecordResponse(ctx, ids)
	}
	return graph, nil
}
