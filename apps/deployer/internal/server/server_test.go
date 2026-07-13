package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"zhblogs-deployer/internal/deploy"
	"zhblogs-deployer/internal/security"
)

type fakeStore struct {
	status string
}

func (store *fakeStore) Create(_ context.Context, _ deploy.WebhookPayload, _ deploy.Classification, _ json.RawMessage) (string, error) {
	return "deployment-id", nil
}

func (store *fakeStore) MarkRunning(_ context.Context, _ string, _ map[string]any) error {
	store.status = "RUNNING"
	return nil
}

func (store *fakeStore) MarkStatus(_ context.Context, _ string, status string, _ map[string]any) error {
	store.status = status
	return nil
}

func (store *fakeStore) MarkFinished(_ context.Context, _ string, status string, _ map[string]any) error {
	store.status = status
	return nil
}

type fakeExecutor struct {
	fail bool
}

func (executor fakeExecutor) Execute(_ context.Context, _ deploy.WebhookPayload, _ deploy.Classification) (deploy.ExecutionResult, error) {
	if executor.fail {
		return deploy.ExecutionResult{}, errTest
	}
	return deploy.ExecutionResult{}, nil
}

func (executor fakeExecutor) ExecuteResume(_ context.Context, _ deploy.WebhookPayload, _ deploy.Classification) (deploy.ExecutionResult, error) {
	return deploy.ExecutionResult{}, nil
}

func (executor fakeExecutor) Rollback(_ context.Context, _ deploy.WebhookPayload) (deploy.ExecutionResult, error) {
	return deploy.ExecutionResult{}, nil
}

var errTest = &testError{}

type testError struct{}

func (*testError) Error() string { return "test error" }

func TestDeployRejectsInvalidSignature(t *testing.T) {
	handler := New(testOptions(&fakeStore{}, fakeExecutor{}))
	request := httptest.NewRequest(http.MethodPost, "/webhooks/deploy", bytes.NewBufferString(`{}`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", response.Code)
	}
}

func TestDeployRecordsSkip(t *testing.T) {
	store := &fakeStore{}
	handler := New(testOptions(store, fakeExecutor{}))
	body := []byte(`{"event":"workflow_push","changed_files":["design/a.md"]}`)
	request := signedRequest(body)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("expected accepted, got %d", response.Code)
	}
	if store.status != "SKIPPED" {
		t.Fatalf("expected skipped, got %s", store.status)
	}
}

func TestDeployRecordsRollback(t *testing.T) {
	store := &fakeStore{}
	handler := New(testOptions(store, fakeExecutor{fail: true}))
	body := []byte(`{"event":"workflow_push","image_tag":"sha","changed_files":["apps/api/index.ts"]}`)
	request := signedRequest(body)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected failure, got %d", response.Code)
	}
	if store.status != "ROLLED_BACK" {
		t.Fatalf("expected rolled back, got %s", store.status)
	}
}

func signedRequest(body []byte) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/webhooks/deploy", bytes.NewBuffer(body))
	request.Header.Set(security.SignatureHeader, security.Sign("secret", body))
	return request
}

func testOptions(store *fakeStore, executor fakeExecutor) Options {
	return Options{
		Secret:   "secret",
		Rules:    testRules(),
		Store:    store,
		Executor: executor,
	}
}

func testRules() deploy.Rules {
	return deploy.Rules{
		Modules: []deploy.ModuleRule{
			{Module: deploy.ModuleAPI, Paths: []string{"apps/api/**"}},
			{Module: deploy.ModuleCloudflare, Paths: []string{"apps/cloudflare/**"}},
		},
	}
}
