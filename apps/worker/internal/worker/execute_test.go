package worker

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"heyblog-worker/internal/config"

	"heyblog.local/edgerequest"
	"heyblog.local/taskprotocol"
)

func TestExecuteResumesCompletedProofDecision(t *testing.T) {
	for _, reason := range []string{"proof_mismatch", "security", "signature", "cancelled"} {
		t.Run(reason, func(t *testing.T) {
			outcome := "UNKNOWN"
			if reason == "proof_mismatch" {
				outcome = "OK"
			}
			// Given a received mismatch whose worker stopped before finishing the target.
			finished := 0
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/internal/v1/tasks/targets/target/finish" {
					t.Errorf("completed proof decision made another request: %s", r.URL.Path)
					rw.WriteHeader(http.StatusConflict)
					return
				}
				finished++
				var input taskprotocol.Finish
				if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
					t.Error(err)
				}
				expected := ""
				if reason != "proof_mismatch" {
					expected = "interrupted"
				}
				if input.Reason != expected {
					t.Errorf("completed decision reported as %q", input.Reason)
				}
				if err := json.NewEncoder(rw).Encode(taskprotocol.ExecutionResult{}); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			w := New(config.Config{APIURL: server.URL, Endpoints: map[edgerequest.Provider]edgerequest.Endpoint{
				edgerequest.EdgeOne: {URL: server.URL, Token: "test-service"},
			}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			defer w.client.CloseIdleConnections()
			work := taskprotocol.Work{ID: "target", Kind: "verification", Address: "https://1.1.1.1/",
				MaxBytes: 512, Configuration: taskprotocol.Configuration{TimeoutSeconds: 8},
				Route: []string{"CF", "EO"}, Progress: map[string]taskprotocol.PathProgress{
					"CF": {Done: true, Path: taskprotocol.PathResult{Outcome: outcome, ErrorKind: reason}},
				}}
			// When the lease is recovered.
			w.execute(t.Context(), work)
			// Then the saved business decision finishes without another authorization or send.
			if finished != 1 {
				t.Fatalf("finish calls=%d, want 1", finished)
			}
		})
	}
}

func TestExecuteUsesBusinessCompletionAcrossRoutes(t *testing.T) {
	for _, tc := range []struct {
		kind, outcome, reason string
		valid                 bool
		requests              int
	}{
		{"verification", "OK", "proof_mismatch", false, 1},
		{"verification", "OK", "", true, 1},
		{"rss", "UNKNOWN", "invalid_feed", false, 2},
		{"rss", "OK", "", true, 1},
		{"monitor", "OK", "", true, 2},
		{"verification", "UNKNOWN", "signature", false, 1},
		{"verification", "UNKNOWN", "security", false, 1},
		{"verification", "UNKNOWN", "cancelled", false, 1},
	} {
		t.Run(tc.kind+"/"+tc.reason, func(t *testing.T) {
			// Given two routes and API-owned business evaluations.
			requests, finished := 0, 0
			edge := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
				requests++
				_, _ = io.WriteString(rw, `{"status":200,"headers":{},"body":""}`)
			}))
			defer edge.Close()
			api := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				var result any
				switch {
				case strings.HasSuffix(r.URL.Path, "/authorize"):
					result = taskprotocol.Permit{ID: "attempt", ExpiresAt: time.Now().Add(time.Minute)}
				case strings.HasSuffix(r.URL.Path, "/sent"):
					result = struct{}{}
				case strings.HasSuffix(r.URL.Path, "/report"):
					result = taskprotocol.Evaluation{Progress: &taskprotocol.PathProgress{Done: true, Valid: tc.valid,
						Path: taskprotocol.PathResult{Outcome: tc.outcome, ErrorKind: tc.reason}}}
				case strings.HasSuffix(r.URL.Path, "/finish"):
					finished++
					result = taskprotocol.ExecutionResult{}
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
				}
				if err := json.NewEncoder(rw).Encode(result); err != nil {
					t.Error(err)
				}
			}))
			defer api.Close()
			w := New(config.Config{APIURL: api.URL, Endpoints: map[edgerequest.Provider]edgerequest.Endpoint{
				edgerequest.Cloudflare: {URL: edge.URL, Token: "test-service"}, edgerequest.EdgeOne: {URL: edge.URL, Token: "test-service"},
			}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			defer w.client.CloseIdleConnections()
			// When a fresh target executes through the HTTP protocol and Edge transport.
			w.execute(t.Context(), taskprotocol.Work{ID: "target", Kind: tc.kind, Address: "https://1.1.1.1/",
				Route: []string{"CF", "EO"}, MaxBytes: 512, Configuration: taskprotocol.Configuration{TimeoutSeconds: 8}})
			// Then completion, fallback and policy stops preserve their request boundaries.
			if requests != tc.requests || finished != 1 {
				t.Fatalf("requests=%d finishes=%d, want %d and 1", requests, finished, tc.requests)
			}
		})
	}
}
