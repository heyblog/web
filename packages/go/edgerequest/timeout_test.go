package edgerequest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type delayedAuthorization struct{}

func (delayedAuthorization) Authorize(ctx context.Context, _ Provider, _, _ string, headers http.Header) (http.Header, error) {
	timer := time.NewTimer(200 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, &Error{Kind: Cancelled, Cause: ctx.Err()}
	case <-timer.C:
		return headers, nil
	}
}

func TestExecute_when_SchedulingWaitExceedsNetworkTimeout(t *testing.T) {
	// Given dispatch pacing longer than the network timeout at each hop.
	var calls atomic.Int32
	client, _ := wireClient(t, func(w http.ResponseWriter, _ *http.Request) {
		response := wireResponse{Status: 200}
		if calls.Add(1) == 1 {
			response = wireResponse{Status: 302, Headers: map[string]string{"location": "/finish"}}
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	})
	client.config.Authorizer = delayedAuthorization{}
	request := probe()
	request.Timeout = 100 * time.Millisecond
	// When both hops complete quickly after their authorization wait.
	result, err := client.Execute(t.Context(), request)
	// Then scheduling delay does not turn a healthy target into a timeout.
	require.NoError(t, err)
	require.Equal(t, 200, result.Status)
	require.Equal(t, 2, result.Requests)
}

func TestExecute_when_ParentDeadlineExpiresDuringScheduling(t *testing.T) {
	// Given an overall operation deadline shorter than dispatch pacing.
	client, _ := wireClient(t, func(http.ResponseWriter, *http.Request) { t.Error("cancelled request dispatched") })
	client.config.Authorizer = delayedAuthorization{}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	// When the task or ownership-verification deadline expires.
	result, err := client.Execute(ctx, probe())
	// Then pacing remains bounded by the caller's total deadline.
	var failure *Error
	require.ErrorAs(t, err, &failure)
	require.Equal(t, Cancelled, failure.Kind)
	require.Zero(t, result.Requests)
}

func TestExecute_when_RedirectsExhaustCombinedNetworkBudget(t *testing.T) {
	// Given two individually fast hops whose combined time exceeds the budget.
	var calls atomic.Int32
	client, _ := wireClient(t, func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		timer := time.NewTimer(300 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
			return
		case <-timer.C:
		}
		response := wireResponse{Status: 200}
		if call == 1 {
			response = wireResponse{Status: 302, Headers: map[string]string{"location": "/finish"}}
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	})
	request := probe()
	request.Timeout = 500 * time.Millisecond
	// When following the redirect, the first hop's time remains consumed.
	result, err := client.Execute(t.Context(), request)
	// Then the second hop cannot obtain a fresh network timeout.
	var failure *Error
	require.ErrorAs(t, err, &failure)
	require.Equal(t, Cancelled, failure.Kind)
	require.Equal(t, 2, result.Requests)
}

type cancelledResolver struct{}

func (cancelledResolver) LookupNetIP(ctx context.Context, _, _ string) ([]netip.Addr, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestExecute_when_CancelledDuringDNS(t *testing.T) {
	// Given a lookup interrupted by the operation's deadline.
	client, _ := wireClient(t, func(http.ResponseWriter, *http.Request) { t.Error("cancelled request dispatched") })
	client.config.Resolver = cancelledResolver{}
	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
	defer cancel()
	// When cancellation happens during target resolution.
	result, err := client.Execute(ctx, probe())
	// Then cancellation cannot be counted as a target health failure.
	var failure *Error
	require.ErrorAs(t, err, &failure)
	require.Equal(t, Cancelled, failure.Kind)
	require.Zero(t, result.Requests)
}
