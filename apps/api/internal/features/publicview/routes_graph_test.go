package publicview

import (
	"context"
	"encoding/json"
	"heyblog-api/internal/platform/httpapi"
	"net/http"
	"net/http/httptest"
	"testing"
)

func (publicViewReaderStub) Graph(context.Context) (FriendGraph, error) {
	return FriendGraph{Nodes: []GraphNode{}, Edges: []GraphEdge{}}, nil
}
func (publicViewReaderStub) SiteGraphByIdentifier(context.Context, SiteIdentifier) (FriendGraph, error) {
	return FriendGraph{Nodes: []GraphNode{}, Edges: []GraphEdge{}}, nil
}

func TestGraphRoutesRequireWebTokenAndDisableCaching(t *testing.T) {
	// Given
	router := newRouterWithViews(t, testHTTPConfig(), publicViewReaderStub{})
	for _, path := range []string{"/api/web/public/sites/graph", "/api/web/public/sites/id/4Aa5Bb6Cc/graph"} {
		t.Run(path, func(t *testing.T) {
			for _, authorized := range []bool{false, true} {
				request := httptest.NewRequest(http.MethodGet, path, nil)
				if authorized {
					request.Header.Set(httpapi.WebTokenHeader, testWebToken)
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
				if authorized {
					var graph FriendGraph
					if err := json.Unmarshal(response.Body.Bytes(), &graph); err != nil || graph.Nodes == nil || graph.Edges == nil {
						t.Fatalf("graph wire contract changed: %s, error = %v", response.Body.String(), err)
					}
					if response.Header().Get("Content-Type") != "application/json" {
						t.Fatal("graph is not JSON")
					}
				}
			}
		})
	}
}
