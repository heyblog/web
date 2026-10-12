package publicview

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"

	"github.com/danielgtaylor/huma/v2"

	"heyblog-api/internal/platform/httpapi"
)

type graphOutput struct {
	CacheControl string `header:"Cache-Control"`
	ContentType  string `header:"Content-Type"`
	Body         []byte
}

func registerGraphRoutes(api huma.API, webToken string, reader Reader) {
	operation := func(id, path, summary string) huma.Operation {
		return huma.Operation{OperationID: id, Method: http.MethodGet, Path: path, Summary: summary,
			Tags: []string{"public views"}, Security: []map[string][]string{{"webToken": {}}},
			Middlewares: huma.Middlewares{httpapi.HumaWebAuthorization(webToken)},
			Errors:      []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError}}
	}
	global := operation("get-friend-graph", "/api/web/public/sites/graph", "Get the public friend-link graph")
	// The cached wire representation has the same typed contract as local graphs.
	global.Responses = map[string]*huma.Response{"200": {Content: map[string]*huma.MediaType{
		"application/json": {Schema: api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[FriendGraph](), true, "")},
	}}}
	httpapi.Register(api, global,
		func(ctx context.Context, _ *publicViewInput) (*graphOutput, error) {
			graph, err := reader.Graph(ctx)
			if err != nil {
				return nil, err
			}
			body := graph.encoded
			if body == nil {
				body, err = json.Marshal(graph)
				if err != nil {
					return nil, internalError(err, "encode friend graph")
				}
			}
			return &graphOutput{CacheControl: "no-store", ContentType: "application/json", Body: body}, nil
		})
	httpapi.Register(api, operation("get-site-friend-graph", "/api/web/public/sites/id/{identifier}/graph", "Get a site's direct friend-link graph"),
		func(ctx context.Context, input *siteIdentifierInput) (*publicViewOutput[FriendGraph], error) {
			identifier, err := parseSiteIdentifier(input.Identifier)
			if err != nil {
				return nil, err
			}
			graph, err := reader.SiteGraphByIdentifier(ctx, identifier)
			if err != nil {
				return nil, err
			}
			return &publicViewOutput[FriendGraph]{CacheControl: "no-store", Body: graph}, nil
		})
}
