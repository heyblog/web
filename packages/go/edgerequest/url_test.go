package edgerequest

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecute_when_TargetNeedsEquivalentURLNormalization(t *testing.T) {
	cases := []struct{ input, expected string }{
		{"https://example.com", "https://example.com/"},
		{"https://EXAMPLE.com:443/feed", "https://example.com/feed"},
		{"http://EXAMPLE.com:80", "http://example.com/"},
		{"http://example.com:443/feed", "http://example.com:443/feed"},
		{"https://example.com/feed?q='中文'&next=%2Fhome+page", "https://example.com/feed?q=%27%E4%B8%AD%E6%96%87%27&next=%2Fhome+page"},
		{"https://example.com/a%20b?x=%2f", "https://example.com/a%20b?x=%2f"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			// Given a valid URL whose wire identity must match native Fetch.
			var sent wireRequest
			client, _ := wireClient(t, func(w http.ResponseWriter, r *http.Request) {
				require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
				require.NoError(t, json.NewEncoder(w).Encode(wireResponse{Status: 200}))
			})
			request := probe()
			request.URL = tc.input
			// When the client validates, authorizes and dispatches the target.
			result, err := client.Execute(t.Context(), request)
			// Then both the proof and the wire use the same canonical URL.
			require.NoError(t, err)
			require.Equal(t, tc.expected, sent.URL)
			require.Equal(t, "fresh-1-"+tc.expected, sent.Headers["X-Heyblog-Proof"])
			require.Equal(t, tc.expected, result.FinalURL)
		})
	}
}

func TestExecute_when_RedirectUsesEquivalentOriginSpelling(t *testing.T) {
	// Given an absolute redirect spelling the same origin with a default port.
	var sent []wireRequest
	client, _ := wireClient(t, func(w http.ResponseWriter, r *http.Request) {
		var request wireRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		sent = append(sent, request)
		response := wireResponse{Status: 200}
		if len(sent) == 1 {
			response = wireResponse{Status: 302, Headers: map[string]string{"location": "https://EXAMPLE.com:443"}}
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	})
	// When the same-origin request follows that redirect.
	result, err := client.Execute(t.Context(), probe())
	// Then its newly authorized hop remains same-origin and Fetch-compatible.
	require.NoError(t, err)
	require.Len(t, sent, 2)
	require.Equal(t, "https://example.com/", sent[1].URL)
	require.Equal(t, "fresh-2-https://example.com/", sent[1].Headers["X-Heyblog-Proof"])
	require.Equal(t, []string{"https://example.com/"}, result.Redirects)
}

func TestExecute_when_PathWouldChangeUnderFetch(t *testing.T) {
	for _, address := range []string{
		"https://example.com/a/%2e%2e/secret",
		"https://example.com/a/../secret",
		`https://example.com/a\secret`,
	} {
		t.Run(address, func(t *testing.T) {
			// Given an ambiguous path which must not be normalized into another resource.
			client, auth := wireClient(t, func(w http.ResponseWriter, _ *http.Request) {
				require.NoError(t, json.NewEncoder(w).Encode(wireResponse{Status: 200}))
			})
			request := probe()
			request.URL = address
			// When the client prepares the request.
			result, err := client.Execute(t.Context(), request)
			// Then rejection precedes signing, quota reservation and transport.
			var failure *Error
			require.ErrorAs(t, err, &failure)
			require.Equal(t, Security, failure.Kind)
			require.Zero(t, result.Requests)
			require.Zero(t, auth.calls.Load())
		})
	}
}
