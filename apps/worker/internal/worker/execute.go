package worker

import (
	"context"
	"errors"
	"net/http"
	"time"

	"heyblog.local/edgerequest"
	"heyblog.local/taskprotocol"
)

func (w *Worker) execute(parent context.Context, work taskprotocol.Work) {
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()
	fence := taskprotocol.Fence{WorkerInput: w.identity(), Token: work.Token}
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				state, err := call[string](ctx, w, "targets/"+work.ID+"/heartbeat", fence)
				if err != nil || state == "cancelling" {
					cancel()
					if state == "cancelling" {
						ack, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
						defer stop()
						if _, err := call[taskprotocol.ExecutionResult](ack, w, "targets/"+work.ID+"/finish",
							taskprotocol.Finish{Fence: fence, Reason: "cancelled"}); err != nil {
							w.logger.Warn("cancellation acknowledgement unavailable", "event", "task_cancel_ack_failed", "target_id", work.ID)
						}
					}
					return
				}
			}
		}
	}()
	defer func() { cancel(); <-done }()
	reason := ""
	released := false
	if work.SampleStartedAt != nil && work.Kind == "monitor" && time.Since(*work.SampleStartedAt) > 5*time.Minute {
		reason = "round_expired"
	} else {
		for _, platform := range work.Route {
			progress := work.Progress[platform]
			if progress.Done {
				if taskprotocol.BusinessComplete(work.Kind, progress) {
					break
				}
				if policyStopped(progress.Path.ErrorKind) {
					reason = "interrupted"
					break
				}
				continue
			}
			session := &requestSession{worker: w, work: work, fence: fence, platform: platform, progress: progress}
			for !session.progress.Done && (session.progress.Pending != nil || session.progress.Attempts <= work.Configuration.Retries) {
				session.retry = session.progress.Attempts
				session.hop, session.parentID = 0, ""
				address := work.Address
				redirects := work.Configuration.MaxRedirects
				if pending := session.progress.Pending; pending != nil {
					address = pending.URL
					session.retry, session.hop, session.parentID = pending.Retry, pending.Hop, pending.ParentID
					redirects -= pending.Hop
				}
				client := edgerequest.New(edgerequest.Config{Endpoints: w.config.Endpoints, Authorizer: session,
					BeforeDispatch: session.beforeSend, Record: session.record})
				_, err := client.Execute(ctx, edgerequest.Request{URL: address, Method: http.MethodGet,
					Platform: edgerequest.Provider(platform), Headers: http.Header{"User-Agent": []string{work.Configuration.UserAgent}},
					MaxBytes: work.MaxBytes, Timeout: time.Duration(work.Configuration.TimeoutSeconds) * time.Second,
					MaxRedirects: redirects, SameOrigin: work.SameOrigin})
				if session.released {
					released = true
					break
				}
				if err != nil && (!session.reported || ctx.Err() != nil) {
					reason = "interrupted"
					break
				}
				if session.progress.Done {
					break
				}
				timer := time.NewTimer(time.Duration(1<<session.retry) * time.Second)
				select {
				case <-ctx.Done():
					timer.Stop()
				case <-timer.C:
				}
				if ctx.Err() != nil {
					break
				}
			}
			if policyStopped(session.progress.Path.ErrorKind) {
				reason = "interrupted"
			}
			if taskprotocol.BusinessComplete(work.Kind, session.progress) {
				break
			}
			if released || ctx.Err() != nil || reason != "" {
				break
			}
		}
	}
	if released || ctx.Err() != nil {
		return
	}
	if _, err := call[taskprotocol.ExecutionResult](ctx, w, "targets/"+work.ID+"/finish", taskprotocol.Finish{Fence: fence, Reason: reason}); err != nil {
		var api *APIError
		if !errors.As(err, &api) || api.Status != http.StatusConflict {
			w.logger.Warn("target completion unavailable", "event", "task_completion_failed", "target_id", work.ID)
		}
	}
}

func policyStopped(kind string) bool {
	switch edgerequest.Kind(kind) {
	case edgerequest.Signature, edgerequest.Security, edgerequest.Cancelled:
		return true
	default:
		return false
	}
}
