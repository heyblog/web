package worker

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"heyblog-worker/internal/config"

	"heyblog.local/taskprotocol"

	"github.com/google/uuid"
)

type Worker struct {
	config   config.Config
	client   *http.Client
	logger   *slog.Logger
	instance taskprotocol.Instance
	ready    atomic.Bool
	draining atomic.Bool
}

func New(c config.Config, logger *slog.Logger) *Worker {
	return &Worker{config: c, logger: logger,
		instance: taskprotocol.Instance{InstanceID: uuid.NewString(), Protocol: taskprotocol.ProtocolVersion, Version: "task-protocol-2"},
		client: &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{MaxIdleConns: 32, MaxIdleConnsPerHost: 20,
			IdleConnTimeout: time.Minute, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 8 * time.Second},
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (w *Worker) identity() taskprotocol.WorkerInput {
	return taskprotocol.WorkerInput{InstanceID: w.instance.InstanceID, Protocol: w.instance.Protocol}
}

func (w *Worker) Run(ctx context.Context) {
	poll := time.NewTicker(5 * time.Second)
	defer poll.Stop()
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	w.run(ctx, poll.C, heartbeat.C)
}

func (w *Worker) run(ctx context.Context, poll, heartbeat <-chan time.Time) {
	executions, stop := context.WithCancel(context.WithoutCancel(ctx))
	defer stop()
	for {
		info, err := call[taskprotocol.Worker](ctx, w, "register", w.instance)
		if err == nil {
			w.ready.Store(true)
			w.draining.Store(info.Draining)
			break
		}
		w.logger.Warn("worker registration unavailable", "event", "worker_registration_failed")
		select {
		case <-ctx.Done():
			return
		case <-poll:
		}
	}
	var workers sync.WaitGroup
	for range 16 {
		workers.Go(func() { w.claimLoop(ctx, executions) })
	}
	workers.Go(func() { w.heartbeatLoop(ctx, heartbeat) })
	for {
		w.schedule(ctx)
		select {
		case <-ctx.Done():
			w.draining.Store(true)
			w.ready.Store(false)
			done := make(chan struct{})
			go func() { workers.Wait(); close(done) }()
			grace := time.NewTimer(25 * time.Second)
			defer grace.Stop()
			select {
			case <-done:
			case <-grace.C:
				stop()
				<-done
			}
			return
		case <-poll:
		}
	}
}

func (w *Worker) heartbeatLoop(ctx context.Context, ticks <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			info, err := call[taskprotocol.Worker](ctx, w, "heartbeat", w.identity())
			w.ready.Store(err == nil)
			if err == nil {
				w.draining.Store(info.Draining)
			}
		}
	}
}

func (w *Worker) schedule(ctx context.Context) {
	if !w.ready.Load() || w.draining.Load() || ctx.Err() != nil {
		return
	}
	due, err := call[taskprotocol.Due](ctx, w, "due", w.identity())
	if err != nil {
		return
	}
	for _, s := range due.Schedules {
		next, err := s.Configuration.Next(due.ServerTime)
		if err != nil {
			continue
		}
		if _, err := call[struct{}](ctx, w, "dispatch", taskprotocol.Dispatch{
			WorkerInput: w.identity(), ID: s.ID, Revision: s.Revision, At: s.At, NextAt: next}); err != nil {
			w.logger.Warn("schedule dispatch deferred", "event", "task_dispatch_deferred", "schedule_id", s.ID)
		}
	}
}

func (w *Worker) claimLoop(acquisition, executions context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for acquisition.Err() == nil {
		if w.ready.Load() && !w.draining.Load() {
			next, err := call[*taskprotocol.Work](acquisition, w, "claim", w.identity())
			if err == nil && next != nil {
				w.execute(executions, *next)
				continue
			}
		}
		select {
		case <-acquisition.Done():
			return
		case <-ticker.C:
		}
	}
}
