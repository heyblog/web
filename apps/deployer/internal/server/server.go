package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"zhblogs-deployer/internal/deploy"
	"zhblogs-deployer/internal/security"
	"zhblogs-deployer/internal/store"
)

type DeploymentExecutor interface {
	Execute(context.Context, deploy.WebhookPayload, deploy.Classification) (deploy.ExecutionResult, error)
	ExecuteResume(context.Context, deploy.WebhookPayload, deploy.Classification) (deploy.ExecutionResult, error)
	Rollback(context.Context, deploy.WebhookPayload) (deploy.ExecutionResult, error)
}

type Options struct {
	Secret     string
	Rules      deploy.Rules
	Store      store.DeploymentStore
	Executor   DeploymentExecutor
	Logger     *log.Logger
	HTTPClient *http.Client
}

func New(options Options) http.Handler {
	mux := http.NewServeMux()
	handler := handler{options: options}
	mux.HandleFunc("GET /health", handler.health)
	mux.HandleFunc("POST /webhooks/deploy", handler.deploy)
	return mux
}

type handler struct {
	options Options
}

func (handler handler) health(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{
		"ok":      true,
		"service": "zhblogs-deployer",
	})
}

func (handler handler) deploy(writer http.ResponseWriter, request *http.Request) {
	body, err := io.ReadAll(io.LimitReader(request.Body, 1<<20))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "READ_BODY_FAILED")
		return
	}

	if !security.Verify(handler.options.Secret, request.Header.Get(security.SignatureHeader), body) {
		writeError(writer, http.StatusUnauthorized, "INVALID_SIGNATURE")
		return
	}

	var payload deploy.WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(writer, http.StatusBadRequest, "INVALID_JSON")
		return
	}
	if payload.Event == "" {
		payload.Event = "workflow_push"
	}

	class := deploy.Classify(payload.ChangedFiles, handler.options.Rules)
	id, err := handler.options.Store.Create(request.Context(), payload, class, json.RawMessage(body))
	if err != nil {
		handler.log("create deployment: %v", err)
		writeError(writer, http.StatusInternalServerError, "CREATE_DEPLOYMENT_FAILED")
		return
	}

	if class.Decision == deploy.DecisionSkip || class.Decision == deploy.DecisionNonServer {
		if payload.HasDeployerAction() {
			class.Decision = deploy.DecisionPartial
		} else {
			status := store.StatusSkipped
			if class.Decision == deploy.DecisionNonServer {
				status = store.StatusSuccess
			}
			handler.finish(request.Context(), id, status, map[string]any{"result": "no server action"})
			writeJSON(writer, http.StatusAccepted, response(id, status, class))
			return
		}
	}

	handler.options.Store.MarkStatus(request.Context(), id, initialStatus(payload, class), map[string]any{"result": "started"})
	result, err := handler.options.Executor.Execute(request.Context(), payload, class)
	if err != nil {
		if errors.Is(err, deploy.ErrWaitingResume) {
			handler.options.Store.MarkStatus(request.Context(), id, store.StatusWaitingResume, map[string]any{"execution": result})
			writeJSON(writer, http.StatusAccepted, response(id, store.StatusWaitingResume, class))
			return
		}
		handler.log("execute deployment: %v", err)
		status, metadata := handler.rollback(request.Context(), payload, result, err)
		handler.finish(request.Context(), id, status, metadata)
		writeJSON(writer, http.StatusInternalServerError, response(id, status, class))
		return
	}

	handler.finish(request.Context(), id, store.StatusSuccess, map[string]any{"execution": result})
	writeJSON(writer, http.StatusAccepted, response(id, store.StatusSuccess, class))
}

func ResumePending(ctx context.Context, options Options) {
	resumableStore, ok := options.Store.(store.ResumableDeploymentStore)
	if !ok {
		return
	}
	records, err := resumableStore.FindWaitingResume(ctx)
	if err != nil {
		if options.Logger != nil {
			options.Logger.Printf("find waiting resume deployments: %v", err)
		}
		return
	}
	for _, record := range records {
		class := deploy.Classify(record.Payload.ChangedFiles, options.Rules)
		if class.Decision == deploy.DecisionSkip || class.Decision == deploy.DecisionNonServer {
			class.Decision = deploy.DecisionPartial
		}
		resumePayload := record.Payload
		resumePayload.NeedsDeployerUpdate = false
		_ = options.Store.MarkStatus(ctx, record.ID, initialStatus(resumePayload, class), map[string]any{"result": "resumed after self update"})
		result, err := options.Executor.ExecuteResume(ctx, record.Payload, class)
		if err != nil {
			status, metadata := handler{options: options}.rollback(ctx, record.Payload, result, err)
			handler{options: options}.finish(ctx, record.ID, status, metadata)
			continue
		}
		handler{options: options}.finish(ctx, record.ID, store.StatusSuccess, map[string]any{"execution": result})
	}
}

func initialStatus(payload deploy.WebhookPayload, class deploy.Classification) string {
	if payload.NeedsDeployerUpdate {
		return store.StatusSelfUpdating
	}
	if payload.NeedsInfraSync {
		return store.StatusRunningInfraSync
	}
	if payload.NeedsDBMigrate {
		return store.StatusRunningDBMigrate
	}
	if len(class.ServerModules) > 0 {
		return store.StatusRunningServices
	}
	return store.StatusRunning
}

func (handler handler) rollback(ctx context.Context, payload deploy.WebhookPayload, result deploy.ExecutionResult, cause error) (string, map[string]any) {
	rollbackResult, rollbackErr := handler.options.Executor.Rollback(ctx, payload)
	metadata := map[string]any{
		"execution": result,
		"error":     cause.Error(),
		"rollback":  rollbackResult,
	}
	if rollbackErr != nil {
		metadata["rollback_error"] = rollbackErr.Error()
		return store.StatusFailed, metadata
	}
	return store.StatusRolledBack, metadata
}

func (handler handler) finish(ctx context.Context, id string, status string, metadata map[string]any) {
	if err := handler.options.Store.MarkFinished(ctx, id, status, metadata); err != nil {
		handler.log("finish deployment: %v", err)
	}
}

func (handler handler) log(format string, args ...any) {
	if handler.options.Logger != nil {
		handler.options.Logger.Printf(format, args...)
	}
}

func response(id string, status string, class deploy.Classification) map[string]any {
	return map[string]any{
		"ok":             status != store.StatusFailed,
		"id":             id,
		"status":         status,
		"classification": class,
	}
}

func writeError(writer http.ResponseWriter, status int, code string) {
	writeJSON(writer, status, map[string]any{
		"ok":    false,
		"error": code,
	})
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("content-type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}
