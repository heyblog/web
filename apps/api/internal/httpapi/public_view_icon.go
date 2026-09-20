package httpapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"heyblog-api/internal/application/publicview"
)

type siteIconOutput struct {
	ContentType  string `header:"Content-Type"`
	CacheControl string `header:"Cache-Control"`
	ETag         string `header:"ETag"`
	Body         []byte
}

func registerSiteIconRoute(api huma.API, webToken string, reader publicview.Reader) {
	Register(api, huma.Operation{
		OperationID: "get-site-icon", Method: http.MethodGet, Path: "/sites/id/{identifier}/icon",
		Summary: "Get a cached site icon as PNG", Tags: []string{"public views"},
		Security: []map[string][]string{{"webToken": {}}}, Middlewares: huma.Middlewares{HumaWebAuthorization(webToken)},
		Errors: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound, http.StatusServiceUnavailable},
		Responses: map[string]*huma.Response{"200": {
			Description: "Normalized PNG, no larger than 128 pixels on either axis",
			Content:     map[string]*huma.MediaType{"image/png": {Schema: &huma.Schema{Type: "string", Format: "binary"}}},
		}},
	}, func(ctx context.Context, input *siteIdentifierInput) (*siteIconOutput, error) {
		identifier, err := parseSiteIdentifier(input.Identifier)
		if err != nil {
			return nil, err
		}
		icon, err := reader.SiteIconByIdentifier(ctx, identifier)
		if err != nil {
			return nil, err
		}
		return &siteIconOutput{ContentType: "image/png", CacheControl: "no-store", ETag: "\"" + icon.Hash + "\"", Body: icon.Content}, nil
	})
}
