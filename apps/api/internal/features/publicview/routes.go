package publicview

import (
	"context"
	"net/http"
	"regexp"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"heyblog-api/internal/domain/site"
	"heyblog-api/internal/platform/apperror"
	"heyblog-api/internal/platform/httpapi"
)

var uuidRoutePattern = regexp.MustCompile(
	`^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$`,
)

type publicViewInput struct{}

type publicViewOutput[T any] struct {
	CacheControl string `header:"Cache-Control"`
	Body         T
}

type directoryInput struct {
	Page         string   `query:"page" doc:"One-based result page"`
	Query        string   `query:"q" doc:"Site name or address search"`
	Level1       string   `query:"level1"`
	Level2       string   `query:"level2"`
	Tertiary     []string `query:"tertiary"`
	Warnings     []string `query:"warning"`
	Technologies []string `query:"technology"`
	Access       []string `query:"access"`
	Feed         string   `query:"feed" enum:"any,with,without"`
	Status       string   `query:"status" enum:"normal,abnormal"`
	Sort         string   `query:"sort" enum:"random,joined,updated"`
	Order        string   `query:"order" enum:"asc,desc"`
	Seed         string   `query:"seed"`
}

type randomSiteInput struct {
	Level1 string `query:"level1" doc:"Optional first-level classification name"`
	Level2 string `query:"level2" doc:"Optional second-level classification name; requires level1"`
}

type siteIdentifierInput struct {
	Identifier string `path:"identifier"`
}

type customSiteIdentifierInput struct {
	CustomID string `path:"customId"`
}

func RegisterRoutes(api huma.API, webToken string, reader Reader) {
	registerSitemapRoute(api, webToken, reader)
	registerAnnouncementReadRoutes(api, webToken, reader)
	registerGraphRoutes(api, webToken, reader)
	registerSiteIconRoute(api, webToken, reader)
	middleware := huma.Middlewares{httpapi.HumaWebAuthorization(webToken)}
	security := []map[string][]string{{"webToken": {}}}
	register := func(operation huma.Operation) huma.Operation {
		operation.Tags = []string{"public views"}
		operation.Security = security
		operation.Middlewares = middleware
		return operation
	}

	httpapi.Register(api, register(huma.Operation{
		OperationID: "get-home",
		Method:      http.MethodGet,
		Path:        "/api/web/public/home",
		Summary:     "Get the home view",
		Errors:      []int{http.StatusUnauthorized, http.StatusServiceUnavailable},
	}), func(ctx context.Context, _ *publicViewInput) (*publicViewOutput[Home], error) {
		view, err := reader.Home(ctx)
		if err != nil {
			return nil, err
		}
		return &publicViewOutput[Home]{CacheControl: "no-store", Body: view}, nil
	})

	httpapi.Register(api, register(huma.Operation{
		OperationID:        "list-sites",
		Method:             http.MethodGet,
		Path:               "/api/web/public/sites",
		Summary:            "List directory sites",
		SkipValidateParams: true,
		Errors:             []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusServiceUnavailable},
	}), func(ctx context.Context, _ *directoryInput) (*publicViewOutput[DirectoryView], error) {
		query, err := parseDirectoryQuery(httpapi.Request(ctx).URL.Query(), time.Now())
		if err != nil {
			return nil, err
		}
		view, err := reader.Directory(ctx, query)
		if err != nil {
			return nil, err
		}
		return &publicViewOutput[DirectoryView]{CacheControl: "no-store", Body: view}, nil
	})

	httpapi.Register(api, register(huma.Operation{
		OperationID:        "get-site-options",
		Method:             http.MethodGet,
		Path:               "/api/web/public/sites/options",
		Summary:            "Get directory filter options",
		SkipValidateParams: true,
		Errors:             []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusServiceUnavailable},
	}), func(ctx context.Context, _ *publicViewInput) (*publicViewOutput[DirectoryOptions], error) {
		for name := range httpapi.Request(ctx).URL.Query() {
			return nil, invalidDirectoryQuery(name, "is not supported")
		}
		view, err := reader.DirectoryOptions(ctx)
		if err != nil {
			return nil, err
		}
		return &publicViewOutput[DirectoryOptions]{CacheControl: "no-store", Body: view}, nil
	})

	httpapi.Register(api, register(huma.Operation{
		OperationID:        "get-random-site",
		Method:             http.MethodGet,
		Path:               "/api/web/public/sites/random",
		Summary:            "Pick a random visible site",
		SkipValidateParams: true,
		Errors:             []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusServiceUnavailable},
	}), func(ctx context.Context, _ *randomSiteInput) (*publicViewOutput[RandomSiteView], error) {
		query, err := parseRandomSiteSearch(httpapi.Request(ctx).URL.RawQuery)
		if err != nil {
			return nil, err
		}
		view, err := reader.RandomSite(ctx, query)
		if err != nil {
			return nil, err
		}
		return &publicViewOutput[RandomSiteView]{CacheControl: "no-store", Body: view}, nil
	})

	httpapi.Register(api, register(huma.Operation{
		OperationID: "get-site-by-identifier",
		Method:      http.MethodGet,
		Path:        "/api/web/public/sites/id/{identifier}",
		Summary:     "Get a site by identifier",
		Errors:      []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound, http.StatusServiceUnavailable},
	}), func(ctx context.Context, input *siteIdentifierInput) (*publicViewOutput[SiteProfile], error) {
		identifier, err := parseSiteIdentifier(input.Identifier)
		if err != nil {
			return nil, err
		}
		view, err := reader.SiteByIdentifier(ctx, identifier)
		if err != nil {
			return nil, err
		}
		return &publicViewOutput[SiteProfile]{CacheControl: "no-store", Body: view}, nil
	})

	httpapi.Register(api, register(huma.Operation{
		OperationID: "get-site-metadata", Method: http.MethodGet,
		Path: "/api/web/public/sites/id/{identifier}/metadata", Summary: "Get site metadata for image generation",
		Errors: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound, http.StatusServiceUnavailable},
	}), func(ctx context.Context, input *siteIdentifierInput) (*publicViewOutput[SiteProfile], error) {
		identifier, err := parseSiteIdentifier(input.Identifier)
		if err != nil {
			return nil, err
		}
		view, err := reader.SiteMetadataByIdentifier(ctx, identifier)
		if err != nil {
			return nil, err
		}
		return &publicViewOutput[SiteProfile]{CacheControl: "no-store", Body: view}, nil
	})

	httpapi.Register(api, register(huma.Operation{
		OperationID: "get-site-by-custom-id",
		Method:      http.MethodGet,
		Path:        "/api/web/public/sites/custom/{customId}",
		Summary:     "Get a site by custom identifier",
		Errors:      []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound, http.StatusServiceUnavailable},
	}), func(ctx context.Context, input *customSiteIdentifierInput) (*publicViewOutput[SiteProfile], error) {
		if err := site.ValidateCustomID(input.CustomID); err != nil {
			return nil, invalidSiteIdentifier("customId")
		}
		view, err := reader.SiteByCustomID(ctx, input.CustomID)
		if err != nil {
			return nil, err
		}
		return &publicViewOutput[SiteProfile]{CacheControl: "no-store", Body: view}, nil
	})
}

func parseSiteIdentifier(value string) (SiteIdentifier, error) {
	if site.ValidateShortID(value) == nil {
		return SiteIdentifier{Kind: IdentifierShortID, Value: value}, nil
	}
	if uuidRoutePattern.MatchString(value) {
		return SiteIdentifier{Kind: IdentifierUUID, Value: value}, nil
	}
	return SiteIdentifier{}, invalidSiteIdentifier("identifier")
}

func invalidSiteIdentifier(name string) error {
	return apperror.New(
		apperror.KindBadRequest,
		apperror.CodeBadRequest,
		"site identifier is invalid",
	).WithInvalidParams([]apperror.InvalidParam{{
		Name: name, Reason: "must use the accepted route format",
	}})
}
