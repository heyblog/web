package httpapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"heyblog-api/internal/features/publicview"
)

func registerGraphRoutes(api huma.API, webToken string, reader publicview.Reader) {
	operation := func(id, path, summary string) huma.Operation {
		return huma.Operation{OperationID: id, Method: http.MethodGet, Path: path, Summary: summary,
			Tags: []string{"public views"}, Security: []map[string][]string{{"webToken": {}}},
			Middlewares: huma.Middlewares{HumaWebAuthorization(webToken)},
			Errors:      []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError}}
	}
	Register(api, operation("get-friend-graph", "/sites/graph", "Get the public friend-link graph"),
		func(ctx context.Context, _ *publicViewInput) (*publicViewOutput[publicview.FriendGraph], error) {
			graph, err := reader.Graph(ctx)
			if err != nil {
				return nil, err
			}
			return &publicViewOutput[publicview.FriendGraph]{CacheControl: "no-store", Body: graph}, nil
		})
	Register(api, operation("get-site-friend-graph", "/sites/id/{identifier}/graph", "Get a site's direct friend-link graph"),
		func(ctx context.Context, input *siteIdentifierInput) (*publicViewOutput[publicview.FriendGraph], error) {
			identifier, err := parseSiteIdentifier(input.Identifier)
			if err != nil {
				return nil, err
			}
			graph, err := reader.SiteGraphByIdentifier(ctx, identifier)
			if err != nil {
				return nil, err
			}
			return &publicViewOutput[publicview.FriendGraph]{CacheControl: "no-store", Body: graph}, nil
		})
}
