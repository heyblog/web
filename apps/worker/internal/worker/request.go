package worker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"heyblog.local/edgerequest"
	"heyblog.local/taskprotocol"
)

var errDeferred = errors.New("target deferred by API")

type requestSession struct {
	worker             *Worker
	work               taskprotocol.Work
	fence              taskprotocol.Fence
	platform           string
	retry, hop         int
	parentID           string
	permit             taskprotocol.Permit
	progress           taskprotocol.PathProgress
	released, reported bool
}

func (s *requestSession) Authorize(ctx context.Context, _ edgerequest.Provider, method, address string, _ http.Header) (http.Header, error) {
	s.reported = false
	for {
		permit, err := call[taskprotocol.Permit](ctx, s.worker, "targets/"+s.work.ID+"/authorize", taskprotocol.Authorization{
			Fence: s.fence, RequestKey: fmt.Sprintf("%s:%d:%d", s.platform, s.retry, s.hop), Platform: s.platform,
			URL: address, Method: method, Retry: s.retry, Hop: s.hop, ParentID: s.parentID})
		if err != nil {
			return nil, &edgerequest.Error{Kind: edgerequest.Signature, Cause: err}
		}
		if permit.RetryAt != nil {
			if permit.Reason == "platform_backoff" || permit.Reason == "request_interval_deferred" {
				s.released = true
				return nil, &edgerequest.Error{Kind: edgerequest.Backoff, Cause: errDeferred}
			}
			timer := time.NewTimer(max(0, time.Until(*permit.RetryAt)))
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if permit.ID == "" || !time.Now().Before(permit.ExpiresAt) {
			return nil, &edgerequest.Error{Kind: edgerequest.Signature, Cause: errDeferred}
		}
		s.permit = permit
		headers := make(http.Header)
		for name, value := range permit.Headers {
			headers.Set(name, string(value))
		}
		return headers, nil
	}
}

func (s *requestSession) beforeSend(ctx context.Context, _ edgerequest.Provider) error {
	_, err := call[struct{}](ctx, s.worker, "targets/"+s.work.ID+"/sent", struct {
		taskprotocol.Fence
		AttemptID string `json:"attempt_id"`
	}{s.fence, s.permit.ID})
	if err != nil {
		return &edgerequest.Error{Kind: edgerequest.Cancelled, Cause: err}
	}
	return nil
}

func (s *requestSession) record(ctx context.Context, observation edgerequest.Observation) error {
	headers := map[string][]byte{}
	for name := range observation.Result.Headers {
		switch http.CanonicalHeaderKey(name) {
		case "Location", "Etag", "Last-Modified", "Retry-After", "Content-Type":
			headers[name] = []byte(observation.Result.Headers.Get(name))
		}
	}
	evaluation, err := call[taskprotocol.Evaluation](ctx, s.worker, "targets/"+s.work.ID+"/report", taskprotocol.Report{
		Fence: s.fence, AttemptID: s.permit.ID, Status: observation.Result.Status, Headers: headers, Body: observation.Result.Body,
		ErrorKind: string(observation.Error), DurationMS: observation.Result.Duration.Milliseconds(), Node: observation.Result.Node})
	if err != nil {
		return err
	}
	s.reported = true
	if evaluation.Progress != nil {
		s.progress = *evaluation.Progress
	}
	s.parentID = s.permit.ID
	s.hop++
	return nil
}
