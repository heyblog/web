//go:build integration

package integration_test

import (
	"fmt"
	"testing"
	"time"

	"heyblog-api/internal/features/taskcenter"

	"heyblog.local/taskprotocol"

	"github.com/stretchr/testify/require"
)

func TestTaskRecoveryCountsOnlyReservedAttempts(t *testing.T) {
	for _, sent := range []bool{false, true} {
		t.Run(map[bool]string{true: "send unconfirmed", false: "authorization unconfirmed"}[sent], func(t *testing.T) {
			// Given an authorized request lost with its process.
			h, w, _ := newFeedTask(t)
			p, err := h.signing.Permit(t.Context(), w.ID, h.authorization(w, w.Route[0], 0))
			require.NoError(t, err)
			if sent {
				require.NoError(t, h.repo.Sent(t.Context(), w.ID, taskcenter.Send{Fence: h.fence(w), AttemptID: p.ID}))
			}
			h.now = h.now.Add(3 * time.Minute)
			_, err = h.repo.WorkerHeartbeat(t.Context(), h.worker)
			require.NoError(t, err)
			// When the target is recovered without resetting the network attempt count.
			recovered, err := h.repo.Claim(t.Context(), h.worker)
			require.NoError(t, err)
			require.NotNil(t, recovered)
			require.Equal(t, 1, recovered.Progress[w.Route[0]].Attempts)
			_, err = h.signing.Permit(t.Context(), recovered.ID, h.authorization(recovered, w.Route[0], 0))
			require.Error(t, err)
			h.report(t, recovered, recovered.Route[1], 0, 200, validTaskFeed, nil)
			result := h.finish(t, recovered)
			// Then unknown delivery remains unknown and only the alternate path succeeds.
			require.True(t, result.BusinessValid)
			var unknown bool
			require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `SELECT unknown FROM operations.request_statistics WHERE attempt_id=$1`, p.ID).Scan(&unknown))
			require.True(t, unknown)
		})
	}
}

func TestMonitorSampleStartsWithFirstConfirmedSend(t *testing.T) {
	// Given a monitoring target that has only been authorized.
	h := newTaskHarness(t, "monitor")
	insertSite(t.Context(), t, h.fixture.pool, "Sample001", "Sample", "sample.example.test")
	_, err := h.fixture.pool.Exec(t.Context(), `UPDATE operations.request_configs SET parameters=jsonb_set(parameters,'{retries}','1') WHERE purpose='monitor'`)
	require.NoError(t, err)
	w := h.start(t)
	first, err := h.signing.Permit(t.Context(), w.ID, h.authorization(w, w.Route[0], 0))
	require.NoError(t, err)
	h.now = h.now.Add(30 * time.Second)
	var unstarted bool
	require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `SELECT sample_started_at IS NULL FROM operations.targets WHERE id=$1`, w.ID).Scan(&unstarted))
	require.True(t, unstarted)

	// When the worker confirms its first send, that confirmation starts sampling.
	h.now = h.now.Add(time.Second)
	require.NoError(t, h.repo.Sent(t.Context(), w.ID, taskcenter.Send{Fence: h.fence(w), AttemptID: first.ID}))
	firstSentAt := h.now
	var sampleStartedAt time.Time
	var roundID string
	require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `SELECT sample_started_at,round_id::text FROM operations.targets WHERE id=$1`, w.ID).Scan(&sampleStartedAt, &roundID))
	require.WithinDuration(t, firstSentAt, sampleStartedAt, time.Microsecond)

	// A second platform send cannot move the round's first-send time.
	second, err := h.signing.Permit(t.Context(), w.ID, h.authorization(w, w.Route[1], 0))
	require.NoError(t, err)
	h.now = h.now.Add(time.Second)
	require.NoError(t, h.repo.Sent(t.Context(), w.ID, taskcenter.Send{Fence: h.fence(w), AttemptID: second.ID}))
	var retainedSample time.Time
	require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `SELECT sample_started_at FROM operations.targets WHERE id=$1`, w.ID).Scan(&retainedSample))
	require.WithinDuration(t, sampleStartedAt, retainedSample, time.Microsecond)

	// Lease recovery retains the same sampling identity and rejects later authorizations after five minutes.
	h.now = h.now.Add(3 * time.Minute)
	_, err = h.repo.WorkerHeartbeat(t.Context(), h.worker)
	require.NoError(t, err)
	recovered, err := h.repo.Claim(t.Context(), h.worker)
	require.NoError(t, err)
	require.NotNil(t, recovered)
	require.NotNil(t, recovered.SampleStartedAt)
	require.WithinDuration(t, sampleStartedAt, *recovered.SampleStartedAt, time.Microsecond)
	var recoveredRoundID string
	require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `SELECT round_id::text FROM operations.targets WHERE id=$1`, recovered.ID).Scan(&recoveredRoundID))
	require.Equal(t, roundID, recoveredRoundID)
	// Keep the recovered lease valid while testing the sampling deadline.
	h.now = sampleStartedAt.Add(4*time.Minute + 59*time.Second)
	_, err = h.repo.WorkerHeartbeat(t.Context(), h.worker)
	require.NoError(t, err)
	_, err = h.repo.Heartbeat(t.Context(), recovered.ID, h.fence(recovered))
	require.NoError(t, err)
	retry := h.authorization(recovered, recovered.Route[0], 1)
	permit, err := h.signing.Permit(t.Context(), recovered.ID, retry)
	require.NoError(t, err)
	require.NotEmpty(t, permit.ID)
	h.now = sampleStartedAt.Add(5*time.Minute + time.Second)
	_, err = h.repo.WorkerHeartbeat(t.Context(), h.worker)
	require.NoError(t, err)
	_, err = h.signing.Permit(t.Context(), recovered.ID, retry)
	require.Error(t, err)
}

func TestStatistics_countLogicalRetriesRedirectHopsAndFallbackTargets(t *testing.T) {
	// Given several observed request hops for one fallback target.
	h := newTaskHarness(t, "monitor")
	insertSite(t.Context(), t, h.fixture.pool, "Stats0001", "Statistics", "statistics.example.test")
	w := h.start(t)
	add := func(retry, hop int, fallback bool, duration int64) {
		t.Helper()
		var attemptID string
		require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `INSERT INTO operations.request_attempts(
			target_id,platform,signed,request_key,kind,url,method,logical_retry,hop,policy_revision,state,created_at)
			VALUES($1,'CF',false,$2,'monitor','https://statistics.example.test/','GET',$3,$4,1,'received',$5)
			RETURNING id::text`, w.ID, fmt.Sprintf("statistics-%d-%d", retry, hop), retry, hop, h.now).Scan(&attemptID))
		_, err := h.fixture.pool.Exec(t.Context(), `INSERT INTO operations.request_statistics(
			attempt_id,platform,kind,created_at,authorized,sent,received,succeeded,duration_ms,fallback)
			VALUES($1,'CF','monitor',$2,true,true,true,true,$3,$4)`, attemptID, h.now, duration, fallback)
		require.NoError(t, err)
	}
	add(1, 0, true, 7)
	add(1, 1, true, 11)
	add(1, 2, true, 13)
	add(2, 0, false, 17)

	// When the API aggregates facts, every hop remains visible while retries and fallbacks are deduplicated.
	statistics, err := h.repo.Stats(t.Context(), taskcenter.Query{})
	require.NoError(t, err)
	require.Len(t, statistics, 1)
	actual := statistics[0]
	require.EqualValues(t, 4, actual.Authorized)
	require.EqualValues(t, 4, actual.Sent)
	require.EqualValues(t, 4, actual.Received)
	require.EqualValues(t, 4, actual.Succeeded)
	require.EqualValues(t, 48, actual.DurationMS)
	require.EqualValues(t, 2, actual.Retries)
	require.EqualValues(t, 2, actual.Redirects)
	require.EqualValues(t, 1, actual.Fallbacks)
}

func TestCancellationRejectsLateBusinessResults(t *testing.T) {
	// Given an already sent request.
	h, w, _ := newFeedTask(t)
	p, err := h.signing.Permit(t.Context(), w.ID, h.authorization(w, w.Route[0], 0))
	require.NoError(t, err)
	require.NoError(t, h.repo.Sent(t.Context(), w.ID, taskcenter.Send{Fence: h.fence(w), AttemptID: p.ID}))
	_, err = h.repo.Control(t.Context(), taskAdministrator, w.RunID, "cancel")
	require.NoError(t, err)
	// When its response races with an accepted cancellation.
	_, err = h.execution.Report(t.Context(), w.ID, taskprotocol.Report{Fence: h.fence(w), AttemptID: p.ID, Status: 200, Body: validTaskFeed})
	// Then no late content is accepted and acknowledgement completes cancellation.
	require.Error(t, err)
	_, err = h.execution.Finish(t.Context(), w.ID, taskprotocol.Finish{Fence: h.fence(w), Reason: "cancelled"})
	require.NoError(t, err)
	var count int
	require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `SELECT count(*) FROM operations.feed_items`).Scan(&count))
	require.Zero(t, count)
}
