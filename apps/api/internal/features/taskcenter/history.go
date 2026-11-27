package taskcenter

import (
	"context"
	"encoding/json"
	"time"
)

type Attempt struct {
	ID             string          `json:"id"`
	TargetID       *string         `json:"target_id"`
	Platform       string          `json:"platform"`
	Signed         bool            `json:"signed"`
	State          string          `json:"state"`
	URL            string          `json:"url"`
	Method         string          `json:"method"`
	Retry          int             `json:"retry"`
	Hop            int             `json:"hop"`
	ParentID       *string         `json:"parent_id"`
	KeyID          *string         `json:"key_id"`
	PolicyRevision int             `json:"policy_revision"`
	CreatedAt      time.Time       `json:"created_at"`
	SentAt         *time.Time      `json:"sent_at"`
	ReceivedAt     *time.Time      `json:"received_at"`
	Accepted       bool            `json:"accepted"`
	Meta           json.RawMessage `json:"meta"`
}
type TaskEvent struct {
	ID        string    `json:"id"`
	ActorID   *string   `json:"actor_id"`
	EntityID  *string   `json:"entity_id"`
	Action    string    `json:"action"`
	Outcome   string    `json:"outcome"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *Repository) Attempts(ctx context.Context, runID string, input Query) (Page[Attempt], error) {
	q, err := normalizeQuery(input)
	page := Page[Attempt]{Items: []Attempt{}, Page: q.Page, PageSize: q.PageSize}
	if err != nil {
		return page, err
	}
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM operations.request_attempts a JOIN operations.targets t ON t.id=a.target_id
		WHERE t.run_id=$1 AND ($2='' OR a.state=$2)`, runID, q.State).Scan(&page.Total); err != nil {
		return page, err
	}
	rows, err := r.db.Query(ctx, `SELECT a.id::text,a.target_id::text,a.platform,a.signed,a.state,a.url,a.method,
		a.logical_retry,a.hop,a.parent_id::text,a.key_id::text,a.policy_revision,a.created_at,a.sent_at,a.received_at,a.accepted,
		a.response_meta-'location' FROM operations.request_attempts a JOIN operations.targets t ON t.id=a.target_id
		WHERE t.run_id=$1 AND ($2='' OR a.state=$2) ORDER BY a.created_at,a.id LIMIT $3 OFFSET $4`,
		runID, q.State, q.PageSize, (q.Page-1)*q.PageSize)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var a Attempt
		if err := rows.Scan(&a.ID, &a.TargetID, &a.Platform, &a.Signed, &a.State, &a.URL, &a.Method, &a.Retry, &a.Hop,
			&a.ParentID, &a.KeyID, &a.PolicyRevision, &a.CreatedAt, &a.SentAt, &a.ReceivedAt, &a.Accepted, &a.Meta); err != nil {
			return page, err
		}
		a.URL = safeAddress(a.URL)
		page.Items = append(page.Items, a)
	}
	return page, rows.Err()
}

func (r *Repository) Events(ctx context.Context, id string, input Query) (Page[TaskEvent], error) {
	q, err := normalizeQuery(input)
	page := Page[TaskEvent]{Items: []TaskEvent{}, Page: q.Page, PageSize: q.PageSize}
	if err != nil {
		return page, err
	}
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM operations.task_events WHERE run_id=$1`, id).Scan(&page.Total); err != nil {
		return page, err
	}
	rows, err := r.db.Query(ctx, `SELECT id::text,actor_id::text,entity_id::text,action,outcome,created_at
		FROM operations.task_events WHERE run_id=$1 ORDER BY created_at,id LIMIT $2 OFFSET $3`, id, q.PageSize, (q.Page-1)*q.PageSize)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var e TaskEvent
		if err := rows.Scan(&e.ID, &e.ActorID, &e.EntityID, &e.Action, &e.Outcome, &e.CreatedAt); err != nil {
			return page, err
		}
		page.Items = append(page.Items, e)
	}
	return page, rows.Err()
}
