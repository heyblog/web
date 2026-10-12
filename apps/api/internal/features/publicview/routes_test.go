package publicview

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"heyblog-api/internal/platform/apperror"
	"heyblog-api/internal/platform/httpapi"
)

func TestPublicViewRoutesUseWebAuthenticationAndTypedReader(t *testing.T) {
	t.Parallel()

	var gotIdentifier SiteIdentifier
	var gotCustomID string
	views := publicViewReaderStub{
		home: func(context.Context) (Home, error) {
			return Home{SiteCount: 2, Sites: []HomeSiteCard{}}, nil
		},
		directory: func(
			_ context.Context,
			query DirectoryQuery,
		) (DirectoryView, error) {
			return DirectoryView{
				Items: []SiteCardView{}, Query: query,
				Pagination: DirectoryPagination{Page: 1, PageSize: 24, TotalPages: 1},
			}, nil
		},
		directoryOptions: func(context.Context) (DirectoryOptions, error) {
			return DirectoryOptions{
				Classifications: []DirectoryClassificationOption{{
					DirectoryOption: DirectoryOption{Value: "technology", Label: "技术", NormalCount: 2},
					Children:        []DirectoryOption{},
				}},
				TertiaryTags: []DirectoryOption{}, Warnings: []DirectoryOption{},
				Technologies: []DirectoryOption{},
			}, nil
		},
		byIdentifier: func(_ context.Context, identifier SiteIdentifier) (SiteProfile, error) {
			gotIdentifier = identifier
			return SiteProfile{SiteCard: SiteCard{ShortID: identifier.Value}}, nil
		},
		byCustomID: func(_ context.Context, customID string) (SiteProfile, error) {
			gotCustomID = customID
			return SiteProfile{SiteCard: SiteCard{CustomID: &customID}}, nil
		},
	}
	router := newRouterWithViews(t, testHTTPConfig(), views)

	tests := []struct {
		path       string
		wantStatus int
		wantBody   string
	}{
		{path: "/api/web/public/home", wantStatus: http.StatusOK, wantBody: `"siteCount":2`},
		{path: "/api/web/public/sites?q=Astro", wantStatus: http.StatusOK, wantBody: `"q":"Astro"`},
		{path: "/api/web/public/sites/options", wantStatus: http.StatusOK, wantBody: `"value":"technology"`},
		{path: "/api/web/public/sites/id/A1b2C3d4E", wantStatus: http.StatusOK, wantBody: `"shortId":"A1b2C3d4E"`},
		{path: "/api/web/public/sites/custom/My_Blog", wantStatus: http.StatusOK, wantBody: `"customId":"My_Blog"`},
	}
	for _, test := range tests {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		request.Header.Set(httpapi.WebTokenHeader, testWebToken)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != test.wantStatus || !strings.Contains(response.Body.String(), test.wantBody) {
			t.Fatalf("%s response = (%d, %q), want status %d containing %q", test.path, response.Code, response.Body.String(), test.wantStatus, test.wantBody)
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s Cache-Control = %q, want no-store", test.path, response.Header().Get("Cache-Control"))
		}
	}
	if gotIdentifier.Kind != IdentifierShortID || gotIdentifier.Value != "A1b2C3d4E" {
		t.Fatalf("identifier = %#v", gotIdentifier)
	}
	if gotCustomID != "My_Blog" {
		t.Fatalf("custom ID = %q", gotCustomID)
	}

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/web/public/home", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}
}

func TestPublicViewRoutesRejectInvalidIdentifiersBeforeApplication(t *testing.T) {
	t.Parallel()

	applicationCalls := 0
	views := publicViewReaderStub{
		byIdentifier: func(context.Context, SiteIdentifier) (SiteProfile, error) {
			applicationCalls++
			return SiteProfile{}, nil
		},
		byCustomID: func(context.Context, string) (SiteProfile, error) {
			applicationCalls++
			return SiteProfile{}, nil
		},
	}
	router := newRouterWithViews(t, testHTTPConfig(), views)
	for _, path := range []string{"/api/web/public/sites/id/not-valid", "/api/web/public/sites/custom/a"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set(httpapi.WebTokenHeader, testWebToken)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"bad_request"`) {
			t.Fatalf("%s response = (%d, %q), want bad request", path, response.Code, response.Body.String())
		}
	}
	if applicationCalls != 0 {
		t.Fatalf("application calls = %d, want 0", applicationCalls)
	}
}

func TestPublicViewRoutePreservesNotFoundProblem(t *testing.T) {
	t.Parallel()

	router := newRouterWithViews(t, testHTTPConfig(), publicViewReaderStub{
		byIdentifier: func(context.Context, SiteIdentifier) (SiteProfile, error) {
			return SiteProfile{}, apperror.New(
				apperror.KindNotFound,
				apperror.CodeNotFound,
				"site was not found",
			)
		},
	})
	request := httptest.NewRequest(http.MethodGet, "/api/web/public/sites/id/A1b2C3d4E", nil)
	request.Header.Set(httpapi.WebTokenHeader, testWebToken)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
		t.Fatalf("response = (%d, %q), want not found", response.Code, response.Body.String())
	}
}

type publicViewReaderStub struct {
	sitemap          func(context.Context, SitemapQuery) (SitemapPage, error)
	home             func(context.Context) (Home, error)
	directory        func(context.Context, DirectoryQuery) (DirectoryView, error)
	directoryOptions func(context.Context) (DirectoryOptions, error)
	randomSite       func(context.Context, RandomSiteQuery) (RandomSiteView, error)
	byIdentifier     func(context.Context, SiteIdentifier) (SiteProfile, error)
	byCustomID       func(context.Context, string) (SiteProfile, error)
}

func (stub publicViewReaderStub) Home(ctx context.Context) (Home, error) {
	if stub.home != nil {
		return stub.home(ctx)
	}
	return Home{Sites: []HomeSiteCard{}}, nil
}

func (stub publicViewReaderStub) Directory(
	ctx context.Context,
	query DirectoryQuery,
) (DirectoryView, error) {
	if stub.directory != nil {
		return stub.directory(ctx, query)
	}
	return DirectoryView{Items: []HomeSiteCard{}}, nil
}

func (stub publicViewReaderStub) DirectoryOptions(
	ctx context.Context,
) (DirectoryOptions, error) {
	if stub.directoryOptions != nil {
		return stub.directoryOptions(ctx)
	}
	return DirectoryOptions{
		Classifications: []DirectoryClassificationOption{}, TertiaryTags: []DirectoryOption{},
		Warnings: []DirectoryOption{}, Technologies: []DirectoryOption{},
	}, nil
}

func (stub publicViewReaderStub) RandomSite(
	ctx context.Context,
	query RandomSiteQuery,
) (RandomSiteView, error) {
	if stub.randomSite != nil {
		return stub.randomSite(ctx, query)
	}
	return RandomSiteView{}, nil
}

func (stub publicViewReaderStub) SiteByIdentifier(
	ctx context.Context,
	identifier SiteIdentifier,
) (SiteProfile, error) {
	if stub.byIdentifier != nil {
		return stub.byIdentifier(ctx, identifier)
	}
	return SiteProfile{}, nil
}

func (stub publicViewReaderStub) SiteByCustomID(
	ctx context.Context,
	customID string,
) (SiteProfile, error) {
	if stub.byCustomID != nil {
		return stub.byCustomID(ctx, customID)
	}
	return SiteProfile{}, nil
}
