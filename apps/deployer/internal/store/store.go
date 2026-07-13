package store

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"zhblogs-deployer/internal/deploy"
)

const (
	StatusPending          = "PENDING"
	StatusRunning          = "RUNNING"
	StatusSelfUpdating     = "SELF_UPDATING"
	StatusWaitingResume    = "WAITING_RESUME"
	StatusRunningInfraSync = "RUNNING_INFRA_SYNC"
	StatusRunningDBMigrate = "RUNNING_DB_MIGRATE"
	StatusRunningServices  = "RUNNING_SERVICES"
	StatusSuccess          = "SUCCESS"
	StatusFailed           = "FAILED"
	StatusRolledBack       = "ROLLED_BACK"
	StatusSkipped          = "SKIPPED"
)

type DeploymentStore interface {
	Create(ctx context.Context, payload deploy.WebhookPayload, class deploy.Classification, raw json.RawMessage) (string, error)
	MarkRunning(ctx context.Context, id string, metadata map[string]any) error
	MarkStatus(ctx context.Context, id string, status string, metadata map[string]any) error
	MarkFinished(ctx context.Context, id string, status string, metadata map[string]any) error
}

type ResumableDeploymentStore interface {
	FindWaitingResume(ctx context.Context) ([]DeploymentRecord, error)
}

type DeploymentRecord struct {
	ID      string
	Payload deploy.WebhookPayload
}

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (store *Store) Close() {
	store.pool.Close()
}

func (store *Store) Create(ctx context.Context, payload deploy.WebhookPayload, class deploy.Classification, raw json.RawMessage) (string, error) {
	metadata := map[string]any{
		"classification": class,
		"image_tag":      payload.ImageTag,
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}

	modules := class.Modules
	if len(modules) == 0 {
		modules = []string{}
	}

	var id string
	err = store.pool.QueryRow(ctx, `
		insert into deployments (
			trigger_event,
			status,
			modules,
			workflow_run_id,
			workflow_run_url,
			commit_sha,
			git_ref,
			metadata,
			raw_payload
		)
		values ($1, $2, $3::deployment_module_enum[], $4, $5, $6, $7, $8::jsonb, $9::jsonb)
		returning id::text
	`, payload.Event, StatusPending, modules, payload.WorkflowRunID, payload.WorkflowRunURL,
		payload.CommitSHA, payload.Ref, metadataJSON, raw).Scan(&id)
	return id, err
}

func (store *Store) MarkRunning(ctx context.Context, id string, metadata map[string]any) error {
	return store.MarkStatus(ctx, id, StatusRunning, metadata)
}

func (store *Store) MarkStatus(ctx context.Context, id string, status string, metadata map[string]any) error {
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	_, err = store.pool.Exec(ctx, `
		update deployments
		set status = $2,
			started_time = coalesce(started_time, $3),
			metadata = metadata || $4::jsonb
		where id = $1
	`, id, status, time.Now(), metadataJSON)
	if err != nil && isDeploymentStatusEnumError(err) {
		metadata["requested_status"] = status
		metadataJSON, marshalErr := json.Marshal(metadata)
		if marshalErr != nil {
			return marshalErr
		}
		_, err = store.pool.Exec(ctx, `
			update deployments
			set status = $2,
				started_time = coalesce(started_time, $3),
				metadata = metadata || $4::jsonb
			where id = $1
		`, id, StatusRunning, time.Now(), metadataJSON)
	}
	return err
}

func (store *Store) MarkFinished(ctx context.Context, id string, status string, metadata map[string]any) error {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	_, err = store.pool.Exec(ctx, `
		update deployments
		set status = $2,
			finished_time = $3,
			metadata = metadata || $4::jsonb
		where id = $1
	`, id, status, time.Now(), metadataJSON)
	return err
}

func (store *Store) FindWaitingResume(ctx context.Context) ([]DeploymentRecord, error) {
	rows, err := store.pool.Query(ctx, `
		select id::text, raw_payload
		from deployments
		where status::text = any($1)
			or metadata->>'requested_status' = any($1)
		order by created_time asc
	`, []string{StatusWaitingResume, StatusSelfUpdating})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]DeploymentRecord, 0)
	for rows.Next() {
		var record DeploymentRecord
		var raw []byte
		if err := rows.Scan(&record.ID, &raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &record.Payload); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func isDeploymentStatusEnumError(err error) bool {
	message := err.Error()
	return strings.Contains(message, "deployment_status_enum") ||
		strings.Contains(message, "invalid input value for enum")
}
