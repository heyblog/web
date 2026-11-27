package worker

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"heyblog-worker/internal/config"

	"heyblog.local/taskprotocol"
)

func TestHeartbeatContinuesDuringScheduleDispatch(t *testing.T) {
	// Given an API whose dispatch remains blocked while other requests succeed.
	dispatch := make(chan struct{})
	heartbeat := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(t.Context())
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		var result any
		switch r.URL.Path {
		case "/api/internal/v1/tasks/register", "/api/internal/v1/tasks/heartbeat":
			result = taskprotocol.Worker{Online: true}
			if r.URL.Path == "/api/internal/v1/tasks/heartbeat" {
				heartbeat <- struct{}{}
			}
		case "/api/internal/v1/tasks/due":
			result = taskprotocol.Due{ServerTime: time.Now(), Schedules: []taskprotocol.DueSchedule{{
				ID: "schedule", Configuration: taskprotocol.Configuration{Mode: "interval", Timezone: "UTC", IntervalSeconds: 60},
			}}}
		case "/api/internal/v1/tasks/dispatch":
			close(dispatch)
			<-r.Context().Done()
			return
		case "/api/internal/v1/tasks/claim":
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(rw).Encode(result)
	}))
	defer server.Close()
	defer cancel()
	w := New(config.Config{APIURL: server.URL}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer w.client.CloseIdleConnections()
	polls, heartbeats := make(chan time.Time), make(chan time.Time, 1)
	done := make(chan struct{})
	go func() { w.run(ctx, polls, heartbeats); close(done) }()
	// When a heartbeat becomes due during the blocked schedule dispatch.
	select {
	case <-dispatch:
	case <-time.After(3 * time.Second):
		t.Fatal("schedule did not dispatch")
	}
	heartbeats <- time.Now()
	// Then the API receives the heartbeat without waiting for dispatch to finish.
	select {
	case <-heartbeat:
	case <-time.After(3 * time.Second):
		t.Error("instance heartbeat blocked behind schedule dispatch")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not shut down")
	}
}

func TestSendingRequiresConfirmedAPIStage(t *testing.T) {
	// Given an API that no longer confirms the sending stage.
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	w := New(config.Config{APIURL: server.URL}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	session := requestSession{worker: w, work: taskprotocol.Work{ID: "target"}, permit: taskprotocol.Permit{ID: "attempt"}}
	// When a send is about to begin.
	err := session.beforeSend(t.Context(), "CF")
	// Then an unavailable API prevents the request from being sent.
	if err == nil {
		t.Fatal("send proceeded without API confirmation")
	}
}
