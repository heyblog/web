package publicview

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"heyblog-api/internal/platform/httpapi"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGraphEncodingPreservesWireContract(t *testing.T) {
	router := newRouterWithViews(t, testHTTPConfig(), publicViewReaderStub{})
	for _, tc := range []struct {
		accept     string
		compressed bool
	}{
		{"gzip", true}, {"br, gzip;q=0.8", true}, {"*;q=1", true},
		{"gzip;q=0, *;q=1", false}, {"identity", false}, {"", false},
	} {
		t.Run(tc.accept, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/web/public/sites/graph", nil)
			request.Header.Set(httpapi.WebTokenHeader, testWebToken)
			request.Header.Set("Accept-Encoding", tc.accept)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d", response.Code)
			}
			if response.Header().Get("Vary") != "Accept-Encoding" {
				t.Fatal("encoding negotiation missing Vary")
			}
			var body io.Reader = response.Body
			if tc.compressed {
				if response.Header().Get("Content-Encoding") != "gzip" {
					t.Fatal("missing gzip encoding")
				}
				reader, err := gzip.NewReader(body)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := reader.Close(); err != nil {
						t.Error(err)
					}
				})
				body = reader
			} else if response.Header().Get("Content-Encoding") != "" {
				t.Fatal("unexpected encoding")
			}
			var graph FriendGraph
			if err := json.NewDecoder(body).Decode(&graph); err != nil {
				t.Fatal(err)
			}
			if graph.Nodes == nil || graph.Edges == nil {
				t.Fatal("graph arrays must remain present")
			}
		})
	}
}

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
