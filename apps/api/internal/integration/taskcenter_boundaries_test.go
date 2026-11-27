//go:build integration

package integration_test

import (
	"net/http"
	"testing"
	"time"

	"heyblog-api/internal/features/taskcenter"

	"heyblog.local/taskprotocol"

	"github.com/stretchr/testify/require"
)

func TestMonitorRejectsIncompleteRedirects(t *testing.T) {
	for _, location := range []string{"", "https://other.example.test/"} {
		t.Run(location, func(t *testing.T) {
			// Given a monitoring path with redirect following disabled.
			h := newTaskHarness(t, "monitor")
			insertSite(t.Context(), t, h.fixture.pool, "Redirect1", "Redirect", "redirect.example.test")
			_, err := h.fixture.pool.Exec(t.Context(), `UPDATE operations.request_configs
				SET parameters=jsonb_set(parameters,'{max_redirects}','0')`)
			require.NoError(t, err)
			w := h.start(t)
			// When the target returns a redirect without an obtainable final result.
			evaluation := h.report(t, w, "EO", 0, http.StatusFound, nil, http.Header{"Location": {location}})
			// Then a redirect cannot be committed as accessible evidence.
			require.False(t, evaluation.Valid)
			require.False(t, evaluation.Continue)
			require.NotNil(t, evaluation.Progress)
			require.NotEqual(t, "OK", evaluation.Progress.Path.Outcome)
			var succeeded, failed int
			require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `SELECT
				count(*) FILTER (WHERE succeeded),count(*) FILTER (WHERE failed)
				FROM operations.request_statistics s JOIN operations.request_attempts a ON a.id=s.attempt_id
				JOIN operations.targets t ON t.id=a.target_id WHERE t.run_id=$1`, w.RunID).Scan(&succeeded, &failed))
			require.Zero(t, succeeded)
			require.Equal(t, 1, failed)
		})
	}
}

func TestPauseBeforeExecutionDoesNotExtendDeadline(t *testing.T) {
	// Given an unstarted queued run paused for one hour.
	h := newTaskHarness(t, "monitor")
	insertSite(t.Context(), t, h.fixture.pool, "Pause0001", "Pause", "pause.example.test")
	id, err := h.repo.Manual(t.Context(), taskAdministrator, h.schedule.ID)
	require.NoError(t, err)
	run, err := h.repo.Control(t.Context(), taskAdministrator, id, "pause")
	require.NoError(t, err)
	require.Equal(t, taskprotocol.Paused, run.State)
	h.now = h.now.Add(time.Hour)
	// When it is resumed before its first target is claimed.
	run, err = h.repo.Control(t.Context(), taskAdministrator, id, "resume")
	require.NoError(t, err)
	// Then queue time cannot become execution-deadline credit.
	require.Zero(t, run.PausedMS)
	require.Nil(t, run.StartedAt)
	_, err = h.repo.Runs(t.Context(), taskcenter.Query{})
	require.NoError(t, err)
}

func TestCancelAcknowledgementSettlesOutstandingAttempts(t *testing.T) {
	for _, sent := range []bool{false, true} {
		t.Run(map[bool]string{false: "authorized", true: "sent"}[sent], func(t *testing.T) {
			// Given an active target with an outstanding request permit.
			h := newTaskHarness(t, "monitor")
			insertSite(t.Context(), t, h.fixture.pool, "Cancel001", "Cancel", "cancel.example.test")
			w := h.start(t)
			permit, err := h.signing.Permit(t.Context(), w.ID, h.authorization(w, "EO", 0))
			require.NoError(t, err)
			if sent {
				require.NoError(t, h.repo.Sent(t.Context(), w.ID, taskcenter.Send{Fence: h.fence(w), AttemptID: permit.ID}))
			}
			_, err = h.repo.Control(t.Context(), taskAdministrator, w.RunID, "cancel")
			require.NoError(t, err)
			// When the worker acknowledges cancellation before a result is confirmed.
			_, err = h.execution.Finish(t.Context(), w.ID, taskprotocol.Finish{Fence: h.fence(w), Reason: "cancelled"})
			require.NoError(t, err)
			// Then the request and persistent counters retain an unknown delivery.
			var state string
			var unknown, received bool
			require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `SELECT a.state,s.unknown,s.received
				FROM operations.request_attempts a JOIN operations.request_statistics s ON s.attempt_id=a.id
				WHERE a.id=$1`, permit.ID).Scan(&state, &unknown, &received))
			require.Equal(t, "unknown", state)
			require.True(t, unknown)
			require.False(t, received)
			_, err = h.execution.Report(t.Context(), w.ID, taskprotocol.Report{
				Fence: h.fence(w), AttemptID: permit.ID, Status: http.StatusOK})
			require.Error(t, err)
		})
	}
}
