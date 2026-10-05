package httpapi

import (
	"context"
	"heyblog-api/internal/features/publicview"
	"net/http"
	"net/http/httptest"
	"testing"
)

func (publicViewReaderStub) Graph(context.Context) (publicview.FriendGraph, error) {
	return publicview.FriendGraph{Nodes: []publicview.GraphNode{}, Edges: []publicview.GraphEdge{}}, nil
}
func (publicViewReaderStub) SiteGraphByIdentifier(context.Context, publicview.SiteIdentifier) (publicview.FriendGraph, error) {
	return publicview.FriendGraph{Nodes: []publicview.GraphNode{}, Edges: []publicview.GraphEdge{}}, nil
}

func TestGraphRoutesRequireWebTokenAndDisableCaching(t *testing.T) {
	// Given
	router := newRouterWithViews(t, testHTTPConfig(), publicViewReaderStub{})
	for _, path := range []string{"/sites/graph", "/sites/id/4Aa5Bb6Cc/graph"} {
		t.Run(path, func(t *testing.T) {
			for _, authorized := range []bool{false, true} {
				request := httptest.NewRequest(http.MethodGet, path, nil)
				if authorized {
					request.Header.Set(WebTokenHeader, testWebToken)
				}
				response := httptest.NewRecorder()
				// When
				router.ServeHTTP(response, request)
				// Then
				want := http.StatusUnauthorized
				if authorized {
					want = http.StatusOK
				}
				if response.Code != want {
					t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
				}
				if authorized && response.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("graph missing no-store")
				}
			}
		})
	}
}
