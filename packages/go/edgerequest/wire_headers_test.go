package edgerequest

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecute_when_HeaderBytesCrossJSON(t *testing.T) {
	// Given a Unicode UA and an opaque ETag containing a non-UTF-8 HTTP byte.
	var requests []struct {
		Headers        map[string]string `json:"headers"`
		HeaderEncoding string            `json:"headerEncoding"`
	}
	client, _ := wireClient(t, func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Headers        map[string]string `json:"headers"`
			HeaderEncoding string            `json:"headerEncoding"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		requests = append(requests, request)
		require.NoError(t, json.NewEncoder(w).Encode(wireResponse{Status: 200, Headers: map[string]string{"etag": "\"\u00e9\""}}))
	})
	request := probe()
	request.Headers.Set("User-Agent", "中文")
	// When a response validator is reused for the next conditional request.
	first, err := client.Execute(t.Context(), request)
	require.NoError(t, err)
	request.Headers.Set("If-None-Match", first.Headers.Get("ETag"))
	_, err = client.Execute(t.Context(), request)
	// Then UTF-8 text and opaque HTTP bytes survive JSON without replacement or double encoding.
	require.NoError(t, err)
	require.Equal(t, "\"\xe9\"", first.Headers.Get("ETag"))
	require.Len(t, requests, 2)
	require.Equal(t, "latin1", requests[0].HeaderEncoding)
	require.Equal(t, "\u00e4\u00b8\u00ad\u00e6\u0096\u0087", requests[0].Headers["User-Agent"])
	require.Equal(t, "\"\u00e9\"", requests[1].Headers["If-None-Match"])
}

func TestExecute_when_EdgeHeaderIsNotAByteString(t *testing.T) {
	// Given a malformed executor response containing a non-byte header value.
	client, _ := wireClient(t, func(w http.ResponseWriter, _ *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(wireResponse{Status: 200, Headers: map[string]string{"etag": "中文"}}))
	})
	// When decoding the executor wire response.
	_, err := client.Execute(t.Context(), probe())
	// Then invalid transport data cannot become a conditional target header.
	var failure *Error
	require.ErrorAs(t, err, &failure)
	require.Equal(t, Platform, failure.Kind)
}
