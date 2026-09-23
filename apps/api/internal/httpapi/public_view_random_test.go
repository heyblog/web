package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"heyblog-api/internal/application/publicview"
)

func TestParseRandomSiteQuery(t *testing.T) {
	t.Parallel()

	query, err := parseRandomSiteQuery(url.Values{"level1": {" 技术 "}, "level2": {"写作"}})
	if err != nil || query.Level1 != "技术" || query.Level2 != "写作" {
		t.Fatalf("parseRandomSiteQuery() = (%#v, %v)", query, err)
	}

	for _, values := range []url.Values{
		{"level1": {""}},
		{"level1": {"  "}},
		{"level1": {"技术"}, "level2": {""}},
		{"preview": {"true"}},
		{"recommend": {"true"}},
		{"level1": {"技术", "生活"}},
		{"level1": {strings.Repeat("文", directoryMaximumFilterValue+1)}},
		{"level2": {"写作"}},
	} {
		if _, err := parseRandomSiteQuery(values); err == nil {
			t.Errorf("parseRandomSiteQuery(%v) accepted invalid query", values)
		}
	}
}

func TestRandomSiteRouteRequiresWebTokenAndReturnsFreshSelection(t *testing.T) {
	t.Parallel()

	called := 0
	router := newRouterWithViews(t, testHTTPConfig(), publicViewReaderStub{
		randomSite: func(_ context.Context, query publicview.RandomSiteQuery) (publicview.RandomSiteView, error) {
			called++
			if query.Level1 != "技术" || query.Level2 != "写作" {
				t.Fatalf("random query = %#v", query)
			}
			return publicview.RandomSiteView{Site: nil}, nil
		},
	})

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/sites/random", nil))
	if unauthorized.Code != http.StatusUnauthorized || called != 0 {
		t.Fatalf("unauthorized response = %d, calls = %d", unauthorized.Code, called)
	}
	for _, path := range []string{"/sites/random?level1=%GG", "/sites/random?level1=foo;bar", "/sites/random?bad=1", "/sites/random?level2=%E5%86%99%E4%BD%9C"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set(WebTokenHeader, testWebToken)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || called != 0 {
			t.Fatalf("invalid random response = %d, calls = %d", response.Code, called)
		}
	}

	for range 2 {
		request := httptest.NewRequest(http.MethodGet, "/sites/random?level1=%E6%8A%80%E6%9C%AF&level2=%E5%86%99%E4%BD%9C", nil)
		request.Header.Set(WebTokenHeader, testWebToken)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" ||
			!strings.Contains(response.Body.String(), `"site":null`) {
			t.Fatalf("random response = (%d, %q, %q)", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
		}
	}
	if called != 2 {
		t.Fatalf("random selection calls = %d, want 2", called)
	}
}
