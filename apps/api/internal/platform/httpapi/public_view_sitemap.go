package httpapi

import (
	"context"
	"net/http"
	"net/url"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"heyblog-api/internal/features/publicview"
	"heyblog-api/internal/platform/apperror"
)

type sitemapInput struct {
	Kind  string `query:"kind" required:"true" enum:"sites,announcements"`
	After string `query:"after" format:"uuid"`
}

func registerSitemapRoute(api huma.API, webToken string, reader publicview.Reader) {
	Register(api, huma.Operation{
		OperationID: "get-sitemap-page", Method: http.MethodGet, Path: "/sitemap",
		Summary: "List indexable site or announcement identifiers for the Web sitemap",
		Tags:    []string{"public views"}, Security: []map[string][]string{{"webToken": {}}},
		Middlewares:        huma.Middlewares{HumaWebAuthorization(webToken)},
		SkipValidateParams: true, Errors: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusInternalServerError},
	}, func(ctx context.Context, _ *sitemapInput) (*publicViewOutput[publicview.SitemapPage], error) {
		values, err := url.ParseQuery(Request(ctx).URL.RawQuery)
		if err != nil {
			return nil, invalidSitemapQuery("query", "must use valid query encoding")
		}
		query, err := parseSitemapQuery(values)
		if err != nil {
			return nil, err
		}
		page, err := reader.Sitemap(ctx, query)
		if err != nil {
			return nil, err
		}
		return &publicViewOutput[publicview.SitemapPage]{CacheControl: "no-store", Body: page}, nil
	})
}

func parseSitemapQuery(values url.Values) (publicview.SitemapQuery, error) {
	for name, entries := range values {
		if name != "kind" && name != "after" {
			return publicview.SitemapQuery{}, invalidSitemapQuery(name, "is not supported")
		}
		if len(entries) != 1 {
			return publicview.SitemapQuery{}, invalidSitemapQuery(name, "must occur exactly once")
		}
	}
	query := publicview.SitemapQuery{Kind: publicview.SitemapKind(values.Get("kind"))}
	switch query.Kind {
	case publicview.SitemapSites, publicview.SitemapAnnouncements:
	default:
		return publicview.SitemapQuery{}, invalidSitemapQuery("kind", "must be sites or announcements")
	}
	if values.Has("after") {
		value := values.Get("after")
		if !uuidRoutePattern.MatchString(value) {
			return publicview.SitemapQuery{}, invalidSitemapQuery("after", "must be a UUID")
		}
		var after pgtype.UUID
		if err := after.Scan(value); err != nil {
			return publicview.SitemapQuery{}, invalidSitemapQuery("after", "must be a UUID")
		}
		query.After = after
	}
	return query, nil
}

func invalidSitemapQuery(name, reason string) error {
	return apperror.New(apperror.KindBadRequest, apperror.CodeBadRequest, "sitemap query is invalid").
		WithInvalidParams([]apperror.InvalidParam{{Name: name, Reason: reason}})
}
