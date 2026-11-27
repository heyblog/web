package edgerequest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExecute_when_CancelledDuringEdgeResponseBody(t *testing.T) {
	// Given an executor that sends headers and then streams an incomplete envelope.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	client, _ := wireClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, err := io.WriteString(w, `{"status":200,"body":"`)
		require.NoError(t, err)
		require.NoError(t, http.NewResponseController(w).Flush())
		cancel()
		<-r.Context().Done()
	})
	// When the caller cancels after the response has started.
	result, err := client.Execute(ctx, probe())
	// Then cancellation keeps its category and cause, without becoming a fallback error.
	var failure *Error
	require.ErrorAs(t, err, &failure)
	require.Equal(t, Cancelled, failure.Kind)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, result.Requests)
}

func TestExecute_when_EdgeResponseBodyIsTruncated(t *testing.T) {
	// Given an executor whose body ends earlier than its Content-Length.
	client, _ := wireClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, err := io.WriteString(w, `{"status":200`)
		require.NoError(t, err)
	})
	// When the response terminates without caller cancellation.
	_, err := client.Execute(t.Context(), probe())
	// Then the broken service transport remains a platform error with its cause.
	var failure *Error
	require.ErrorAs(t, err, &failure)
	require.Equal(t, Platform, failure.Kind)
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

func TestExecute_when_EdgeReportsTargetTimeout(t *testing.T) {
	// Given an executor with transport overhead before its target timeout begins.
	client, _ := wireClient(t, func(w http.ResponseWriter, r *http.Request) {
		var wire wireRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&wire))
		timer := time.NewTimer(time.Duration(wire.TimeoutMS)*time.Millisecond + 20*time.Millisecond)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
			return
		case <-timer.C:
		}
		require.NoError(t, json.NewEncoder(w).Encode(wireResponse{Error: "target_timeout"}))
	})
	request := probe()
	request.Timeout = 400 * time.Millisecond
	// When the target times out within the overall network allowance.
	result, err := client.Execute(t.Context(), request)
	// Then the explicit target failure arrives before the service transport expires.
	var failure *Error
	require.ErrorAs(t, err, &failure)
	require.Equal(t, Target, failure.Kind)
	require.Equal(t, 1, result.Requests)
}
