//go:build integration

package integration_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTaskDispatchIntervalCoversRepeatedAttemptsAndPlatformChanges(t *testing.T) {
	// Given two paths sharing persisted run pacing.
	h := newTaskHarness(t, "monitor")
	insertSite(t.Context(), t, h.fixture.pool, "PacedSite", "Paced site", "paced.example.test")
	configs, err := h.repo.Configurations(t.Context())
	require.NoError(t, err)
	for _, c := range configs {
		if c.Purpose == "monitor" {
			c.Parameters.IntervalMS = 200
			_, err = h.repo.SaveConfiguration(t.Context(), taskAdministrator, c.ID, c.RequestConfigInput)
			require.NoError(t, err)
		}
	}
	w := h.start(t)
	first := h.authorization(w, "EO", 0)
	p, err := h.signing.Permit(t.Context(), w.ID, first)
	require.NoError(t, err)
	// When an authorization is retransmitted and another path requests permission.
	repeated, err := h.signing.Permit(t.Context(), w.ID, first)
	require.NoError(t, err)
	require.Equal(t, p.ID, repeated.ID)
	second, err := h.signing.Permit(t.Context(), w.ID, h.authorization(w, "CF", 0))
	require.NoError(t, err)
	// Then retransmission consumes no second attempt and pacing crosses platforms.
	require.Empty(t, second.ID)
	require.NotNil(t, second.RetryAt)
	require.Equal(t, "request_interval", second.Reason)
	h.now = h.now.Add(200 * time.Millisecond)
	second, err = h.signing.Permit(t.Context(), w.ID, h.authorization(w, "CF", 0))
	require.NoError(t, err)
	require.NotEmpty(t, second.ID)
	var count int
	require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `SELECT count(*) FROM operations.request_attempts WHERE target_id=$1`, w.ID).Scan(&count))
	require.Equal(t, 2, count)
}
