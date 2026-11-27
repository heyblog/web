package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type APIError struct{ Status int }

func (e *APIError) Error() string { return fmt.Sprintf("task API status %d", e.Status) }

func call[T any](ctx context.Context, w *Worker, path string, body any) (T, error) {
	var result T
	payload, err := json.Marshal(body)
	if err != nil {
		return result, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, w.config.APIURL+"/api/internal/v1/tasks/"+path, bytes.NewReader(payload))
	if err != nil {
		return result, err
	}
	request.Header.Set("Authorization", "Bearer "+w.config.Token)
	request.Header.Set("Content-Type", "application/json")
	response, err := w.client.Do(request)
	if err != nil {
		w.ready.Store(false)
		return result, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return result, &APIError{Status: response.StatusCode}
	}
	err = json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&result)
	if err != nil {
		w.ready.Store(false)
	}
	return result, err
}
