package publicview

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"heyblog-api/internal/platform/httpapi"
)

func TestSitemapReturnsTypedPageWhenWebAuthorized(t *testing.T) {
	t.Parallel()
	for _, kind := range []SitemapKind{SitemapSites, SitemapAnnouncements} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			// Given an authorized reader returning kind-specific indexable metadata.
			id := "0199abcd-0000-7000-8000-000000000001"
			published := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
			item := SitemapItem{ID: id, ShortID: "A1b2C3d4E"}
			if kind == SitemapAnnouncements {
				item = SitemapItem{ID: id, StartsAt: &published, PublishedAt: &published, UpdatedAt: &published}
			}
			var got SitemapQuery
			router := newRouterWithViews(t, testHTTPConfig(), publicViewReaderStub{sitemap: func(_ context.Context, query SitemapQuery) (SitemapPage, error) {
				got = query
				return SitemapPage{Items: []SitemapItem{item}, NextAfter: &id}, nil
			}})
			request := httptest.NewRequest(http.MethodGet, "/api/web/public/sitemap?kind="+string(kind)+"&after="+id, nil)
			request.Header.Set(httpapi.WebTokenHeader, testWebToken)
			response := httptest.NewRecorder()
			// When the sitemap page is requested.
			router.ServeHTTP(response, request)
			// Then the cursor and item fields are preserved without response caching.
			if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("response = (%d, %q)", response.Code, response.Body.String())
			}
			if got.Kind != kind || got.After.String() != id {
				t.Fatalf("query = %#v", got)
			}
			var page SitemapPage
			if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
				t.Fatal(err)
			}
			if len(page.Items) != 1 || page.Items[0].ID != id || page.NextAfter == nil || *page.NextAfter != id {
				t.Fatalf("page = %#v", page)
			}
			if kind == SitemapSites && strings.Contains(response.Body.String(), "publishedAt") || kind == SitemapAnnouncements && strings.Contains(response.Body.String(), "shortId") {
				t.Fatal("response contains metadata from another sitemap kind")
			}
		})
	}
}

func TestSitemapRejectsInvalidQueryBeforeReader(t *testing.T) {
	t.Parallel()
	for _, query := range []string{
		"", "kind=other", "kind=sites&kind=announcements", "kind=sites&after=", "kind=sites&after=bad",
		"kind=sites&after=0199abcd000070008000000000000001", "kind=sites&after=0199abcd-0000-7000-8000-000000000001&after=x",
		"kind=sites&page=1", "kind=sites&after=%ZZ", "kind=sites;x=1",
	} {
		t.Run(query, func(t *testing.T) {
			t.Parallel()
			// Given malformed query parameters and a reader which must remain unused.
			router := newRouterWithViews(t, testHTTPConfig(), publicViewReaderStub{sitemap: func(context.Context, SitemapQuery) (SitemapPage, error) {
				t.Fatal("reader called for an invalid query")
				return SitemapPage{}, nil
			}})
			request := httptest.NewRequest(http.MethodGet, "/api/web/public/sitemap?"+query, nil)
			request.Header.Set(httpapi.WebTokenHeader, testWebToken)
			response := httptest.NewRecorder()
			// When the sitemap endpoint receives the query.
			router.ServeHTTP(response, request)
			// Then validation returns the stable Bad httpapi.Request problem.
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"bad_request"`) {
				t.Fatalf("response = (%d, %q)", response.Code, response.Body.String())
			}
		})
	}
}

func TestSitemapRejectsMissingWebTokenBeforeReader(t *testing.T) {
	t.Parallel()
	// Given a reader that may only be called after Web authorization.
	router := newRouterWithViews(t, testHTTPConfig(), publicViewReaderStub{sitemap: func(context.Context, SitemapQuery) (SitemapPage, error) {
		t.Fatal("unauthorized sitemap read")
		return SitemapPage{}, nil
	}})
	response := httptest.NewRecorder()
	// When the endpoint is accessed without a Web token.
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/web/public/sitemap?kind=sites", nil))
	// Then authorization rejects the request.
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestSitemapFailureDoesNotExposeInternalDetails(t *testing.T) {
	t.Parallel()
	// Given a failed database-backed read.
	router := newRouterWithViews(t, testHTTPConfig(), publicViewReaderStub{sitemap: func(context.Context, SitemapQuery) (SitemapPage, error) {
		return SitemapPage{}, errors.New("private database failure")
	}})
	request := httptest.NewRequest(http.MethodGet, "/api/web/public/sitemap?kind=sites", nil)
	request.Header.Set(httpapi.WebTokenHeader, testWebToken)
	response := httptest.NewRecorder()
	// When the read failure reaches the HTTP boundary.
	router.ServeHTTP(response, request)
	// Then it returns an internal problem without underlying diagnostics.
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "private database") {
		t.Fatalf("response = (%d, %q)", response.Code, response.Body.String())
	}
}

func TestSitemapEmptyCollectionIsSuccessful(t *testing.T) {
	t.Parallel()
	// Given an authorized reader with no indexable pages.
	router := newTestRouter(t, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/web/public/sitemap?kind=sites", nil)
	request.Header.Set(httpapi.WebTokenHeader, testWebToken)
	response := httptest.NewRecorder()
	// When the collection is requested.
	router.ServeHTTP(response, request)
	// Then clients receive an empty array and a terminal null cursor.
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"items":[]`) || !strings.Contains(response.Body.String(), `"nextAfter":null`) {
		t.Fatalf("response = (%d, %q)", response.Code, response.Body.String())
	}
}

func (stub publicViewReaderStub) Sitemap(ctx context.Context, query SitemapQuery) (SitemapPage, error) {
	if stub.sitemap != nil {
		return stub.sitemap(ctx, query)
	}
	return SitemapPage{Items: []SitemapItem{}}, nil
}
