//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"heyblog-api/internal/features/taskcenter"

	"heyblog.local/taskprotocol"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var taskAdministrator = taskcenter.Actor{System: true}

type taskHarness struct {
	fixture   auditMigrationFixture
	repo      *taskcenter.Repository
	signing   *taskcenter.Signing
	execution *taskcenter.Execution
	worker    taskprotocol.WorkerInput
	now       time.Time
	schedule  taskcenter.Schedule
}

func newTaskHarness(t *testing.T, kind string) *taskHarness {
	t.Helper()
	h := &taskHarness{fixture: newAuditMigrationFixture(t), now: time.Now().UTC()}
	_, err := h.fixture.provider.Up(t.Context())
	require.NoError(t, err)
	h.repo = taskcenter.NewRepository(h.fixture.pool, func() time.Time { return h.now })
	h.signing = taskcenter.NewSigning(h.repo, make([]byte, 32))
	h.execution = taskcenter.NewExecution(h.repo)
	h.worker = taskprotocol.WorkerInput{InstanceID: uuid.NewString(), Protocol: taskprotocol.ProtocolVersion}
	_, err = h.repo.Register(t.Context(), taskprotocol.Instance{InstanceID: h.worker.InstanceID, Protocol: h.worker.Protocol, Version: "integration"})
	require.NoError(t, err)
	configs, err := h.repo.Configurations(t.Context())
	require.NoError(t, err)
	for _, c := range configs {
		c.Parameters.Signed = false
		c.Parameters.IntervalMS = 0
		_, err = h.repo.SaveConfiguration(t.Context(), taskAdministrator, c.ID, c.RequestConfigInput)
		require.NoError(t, err)
	}
	schedules, err := h.repo.Schedules(t.Context())
	require.NoError(t, err)
	for _, s := range schedules {
		if s.Kind == kind {
			h.schedule = s
		}
	}
	return h
}

func (h *taskHarness) start(t *testing.T) *taskprotocol.Work {
	t.Helper()
	h.now = time.Now().UTC().Add(time.Second)
	_, err := h.repo.Manual(t.Context(), taskAdministrator, h.schedule.ID)
	require.NoError(t, err)
	w, err := h.repo.Claim(t.Context(), h.worker)
	require.NoError(t, err)
	require.NotNil(t, w)
	return w
}

func (h *taskHarness) authorization(w *taskprotocol.Work, platform string, retry int) taskprotocol.Authorization {
	return taskprotocol.Authorization{Fence: h.fence(w), RequestKey: fmt.Sprintf("%s:%d:0", platform, retry),
		Platform: platform, URL: w.Address, Method: http.MethodGet, Retry: retry}
}

func (h *taskHarness) fence(w *taskprotocol.Work) taskprotocol.Fence {
	return taskprotocol.Fence{WorkerInput: h.worker, Token: w.Token}
}

func (h *taskHarness) report(t *testing.T, w *taskprotocol.Work, platform string, retry int, status int, body []byte, headers http.Header) taskprotocol.Evaluation {
	t.Helper()
	permit, err := h.signing.Permit(t.Context(), w.ID, h.authorization(w, platform, retry))
	require.NoError(t, err)
	require.NotEmpty(t, permit.ID)
	require.NoError(t, h.repo.Sent(t.Context(), w.ID, taskcenter.Send{Fence: h.fence(w), AttemptID: permit.ID}))
	rawHeaders := map[string][]byte{}
	for name := range headers {
		rawHeaders[name] = []byte(headers.Get(name))
	}
	report := taskprotocol.Report{Fence: h.fence(w), AttemptID: permit.ID, Status: status, Body: body, Headers: rawHeaders, Node: map[string]string{}}
	evaluation, err := h.execution.Report(t.Context(), w.ID, report)
	require.NoError(t, err)
	repeated, err := h.execution.Report(t.Context(), w.ID, report)
	require.NoError(t, err)
	require.Equal(t, evaluation, repeated)
	return evaluation
}

func (h *taskHarness) finish(t *testing.T, w *taskprotocol.Work) taskprotocol.ExecutionResult {
	t.Helper()
	result, err := h.execution.Finish(t.Context(), w.ID, taskprotocol.Finish{Fence: h.fence(w)})
	require.NoError(t, err)
	repeated, err := h.execution.Finish(t.Context(), w.ID, taskprotocol.Finish{Fence: h.fence(w)})
	require.NoError(t, err)
	require.Equal(t, result, repeated)
	return result
}
