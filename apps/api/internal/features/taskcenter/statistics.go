package taskcenter

import (
	"context"
	"time"

	"heyblog.local/taskprotocol"
)

type Statistics struct {
	Platform   string `json:"platform"`
	Kind       string `json:"kind"`
	Authorized int64  `json:"authorized"`
	Sent       int64  `json:"sent"`
	Received   int64  `json:"received"`
	Succeeded  int64  `json:"succeeded"`
	Failed     int64  `json:"failed"`
	Unknown    int64  `json:"unknown"`
	DurationMS int64  `json:"duration_ms"`
	Retries    int64  `json:"retries"`
	Redirects  int64  `json:"redirects"`
	Fallbacks  int64  `json:"fallbacks"`
}
type Backoff struct {
	Platform string     `json:"platform"`
	RetryAt  *time.Time `json:"retry_at"`
	Reason   string     `json:"reason"`
	Stage    int        `json:"stage"`
}
type Overview struct {
	States  map[string]int        `json:"states"`
	Recent  []Run                 `json:"recent"`
	Workers []taskprotocol.Worker `json:"workers"`
	Backoff []Backoff             `json:"backoff"`
}

func (r *Repository) Stats(ctx context.Context, input Query) ([]Statistics, error) {
	q, err := normalizeQuery(input)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `SELECT s.platform,s.kind,count(*) FILTER(WHERE s.authorized),count(*) FILTER(WHERE s.sent),
		count(*) FILTER(WHERE s.received),count(*) FILTER(WHERE s.succeeded),count(*) FILTER(WHERE s.failed),
		count(*) FILTER(WHERE s.unknown),COALESCE(sum(s.duration_ms),0),
		count(DISTINCT (a.target_id,a.platform,a.logical_retry)) FILTER(WHERE a.logical_retry>0),
		count(*) FILTER(WHERE a.hop>0),count(DISTINCT a.target_id) FILTER(WHERE s.fallback)
		FROM operations.request_statistics s JOIN operations.request_attempts a ON a.id=s.attempt_id
		WHERE ($1='' OR s.kind=$1) AND ($2='' OR s.created_at>=NULLIF($2,'')::timestamptz)
		AND ($3='' OR s.created_at<=NULLIF($3,'')::timestamptz) GROUP BY s.platform,s.kind ORDER BY s.platform,s.kind`, q.Kind, q.From, q.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Statistics{}
	for rows.Next() {
		var s Statistics
		if err := rows.Scan(&s.Platform, &s.Kind, &s.Authorized, &s.Sent, &s.Received, &s.Succeeded, &s.Failed,
			&s.Unknown, &s.DurationMS, &s.Retries, &s.Redirects, &s.Fallbacks); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

func (r *Repository) Overview(ctx context.Context) (Overview, error) {
	result := Overview{States: map[string]int{}, Recent: []Run{}, Backoff: []Backoff{}}
	rows, err := r.db.Query(ctx, `SELECT state,count(*) FROM operations.runs WHERE NOT archived GROUP BY state`)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var state string
		var count int
		if err := rows.Scan(&state, &count); err != nil {
			rows.Close()
			return result, err
		}
		result.States[state] = count
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return result, err
	}
	page, err := r.Runs(ctx, Query{Page: 1, PageSize: 5, State: "failed"})
	if err != nil {
		return result, err
	}
	result.Recent = page.Items
	result.Workers, err = r.Workers(ctx)
	if err != nil {
		return result, err
	}
	rows, err = r.db.Query(ctx, `SELECT platform,retry_at,reason,stage FROM operations.platform_backoff ORDER BY platform`)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var b Backoff
		if err := rows.Scan(&b.Platform, &b.RetryAt, &b.Reason, &b.Stage); err != nil {
			return result, err
		}
		result.Backoff = append(result.Backoff, b)
	}
	return result, rows.Err()
}
