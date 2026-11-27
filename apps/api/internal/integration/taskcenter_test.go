//go:build integration

package integration_test

import (
	"sync"
	"testing"
	"time"

	"heyblog-api/internal/features/monitoring"
	"heyblog-api/internal/features/taskcenter"

	"heyblog.local/taskprotocol"

	"github.com/stretchr/testify/require"
)

func TestTaskCenterConcurrentClaimsRecoveryAndAggregation(t *testing.T) {
	// Given one due schedule and a visible site.
	h := newTaskHarness(t, "monitor")
	siteID := integrationUUIDText(t, insertSite(t.Context(), t, h.fixture.pool, "TaskCheck", "Task check", "task.example.test"))
	h.schedule.Configuration.Scope = "selected"
	h.schedule.Configuration.SiteIDs = []string{siteID}
	h.schedule.Enabled = true
	var err error
	h.schedule, err = h.repo.Save(t.Context(), taskAdministrator, h.schedule.ID, h.schedule.ScheduleInput)
	require.NoError(t, err)
	dueAt := *h.schedule.NextAt
	h.now = h.now.Add(2 * time.Hour)
	_, err = h.repo.WorkerHeartbeat(t.Context(), h.worker)
	require.NoError(t, err)
	due, err := h.repo.Due(t.Context(), h.worker)
	require.NoError(t, err)
	require.Len(t, due.Schedules, 1)
	next, err := h.schedule.Configuration.Next(h.now)
	require.NoError(t, err)
	var wg sync.WaitGroup
	// When several workers race to dispatch and acquire.
	for range 8 {
		wg.Go(func() {
			_, _ = h.repo.Dispatch(t.Context(), taskprotocol.Dispatch{WorkerInput: h.worker, ID: h.schedule.ID,
				Revision: h.schedule.Revision, At: dueAt, NextAt: next})
		})
	}
	wg.Wait()
	runs, err := h.repo.Runs(t.Context(), taskcenter.Query{})
	require.NoError(t, err)
	require.Len(t, runs.Items, 1)
	leases := make(chan *taskprotocol.Work, 8)
	for range 8 {
		wg.Go(func() {
			w, err := h.repo.Claim(t.Context(), h.worker)
			if err != nil {
				t.Error(err)
				return
			}
			if w != nil {
				leases <- w
			}
		})
	}
	wg.Wait()
	close(leases)
	var work []*taskprotocol.Work
	for w := range leases {
		work = append(work, w)
	}
	require.Len(t, work, 1)
	first := work[0]
	h.now = h.now.Add(3 * time.Minute)
	_, err = h.repo.WorkerHeartbeat(t.Context(), h.worker)
	require.NoError(t, err)
	recovered, err := h.repo.Claim(t.Context(), h.worker)
	require.NoError(t, err)
	require.NotNil(t, recovered)
	require.NotEqual(t, first.Token, recovered.Token)
	_, err = h.repo.Heartbeat(t.Context(), first.ID, h.fence(first))
	require.Error(t, err)
	h.report(t, recovered, "EO", 0, 200, nil, nil)
	h.report(t, recovered, "CF", 0, 200, nil, nil)
	h.finish(t, recovered)
	// Then actual observation time is counted once, and changed addresses reject old samples.
	var samples int
	var sampled time.Time
	require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `SELECT count(*),min(round_at) FROM directory.site_monitoring_rounds WHERE site_id=$1`, siteID).Scan(&samples, &sampled))
	require.Equal(t, 1, samples)
	require.WithinDuration(t, h.now, sampled, time.Microsecond)
	aggregator := monitoring.New(h.fixture.pool)
	for round := 1; round < 10; round++ {
		require.NoError(t, aggregator.Record(t.Context(), monitoring.Round{SiteID: siteID, Generation: 1, ID: time.Duration(round).String(),
			StartedAt: h.now.Add(time.Duration(round) * time.Minute), CN: monitoring.OK, GL: monitoring.Fail}))
	}
	var health, scope *string
	require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `SELECT status,access_scope FROM directory.sites WHERE id=$1`, siteID).Scan(&health, &scope))
	require.Equal(t, "OK", *health)
	require.Equal(t, "CN", *scope)
	_, err = h.fixture.pool.Exec(t.Context(), `UPDATE directory.sites SET normalized_host='changed.example.test' WHERE id=$1`, siteID)
	require.NoError(t, err)
	require.ErrorIs(t, aggregator.Record(t.Context(), monitoring.Round{SiteID: siteID, Generation: 1, ID: "stale", StartedAt: h.now, CN: monitoring.OK, GL: monitoring.OK}), monitoring.ErrStaleGeneration)
}

func TestTaskControlDrainsPausesAndCancels(t *testing.T) {
	// Given a running target and a pending target.
	h := newTaskHarness(t, "monitor")
	insertSite(t.Context(), t, h.fixture.pool, "Control01", "Control one", "control-one.example.test")
	insertSite(t.Context(), t, h.fixture.pool, "Control02", "Control two", "control-two.example.test")
	w := h.start(t)
	// When pause is requested, acquisition stops until the leased target drains.
	run, err := h.repo.Control(t.Context(), taskAdministrator, w.RunID, "pause")
	require.NoError(t, err)
	require.Equal(t, taskprotocol.Pausing, run.State)
	other, err := h.repo.Claim(t.Context(), h.worker)
	require.NoError(t, err)
	require.Nil(t, other)
	h.report(t, w, "EO", 0, 200, nil, nil)
	h.report(t, w, "CF", 0, 503, nil, nil)
	h.finish(t, w)
	runs, err := h.repo.Runs(t.Context(), taskcenter.Query{})
	require.NoError(t, err)
	require.Equal(t, taskprotocol.Paused, runs.Items[0].State)
	h.now = h.now.Add(time.Hour)
	run, err = h.repo.Control(t.Context(), taskAdministrator, w.RunID, "resume")
	require.NoError(t, err)
	require.Equal(t, int64(time.Hour/time.Millisecond), run.PausedMS)
	run, err = h.repo.Control(t.Context(), taskAdministrator, w.RunID, "cancel")
	require.NoError(t, err)
	// Then completed evidence survives cancellation and pending work is never sent.
	require.Equal(t, taskprotocol.Cancelled, run.State)
	require.Equal(t, 1, run.Counts.Succeeded)
	require.Equal(t, 1, run.Counts.Cancelled)
}
