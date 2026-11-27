package taskcenter

import (
	"context"
	"errors"
	"net/url"
	"time"

	"heyblog.local/taskprotocol"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const runColumns = `r.id::text,r.schedule_id::text,r.kind,r.state,r.snapshot,r.scheduled_at,r.created_at,
	r.started_at,r.finished_at,r.paused_at,r.paused_ms,r.timeout_seconds,r.source,r.actor_id::text,
	r.root_run_id::text,r.parent_run_id::text,r.retry_number,r.archived,r.cleaned_at,r.reason`

const countColumns = `count(*),count(*) FILTER(WHERE state='succeeded'),
	count(*) FILTER(WHERE state IN ('failed','partial_failed','timed_out')),
	count(*) FILTER(WHERE state='skipped'),count(*) FILTER(WHERE state='cancelled'),
	count(*) FILTER(WHERE state IN ('pending','running','retry_wait')),count(*) FILTER(WHERE usable)`

func scanRun(row pgx.Row) (Run, error) {
	var r Run
	err := row.Scan(&r.ID, &r.ScheduleID, &r.Kind, &r.State, &r.Snapshot, &r.ScheduledAt, &r.CreatedAt,
		&r.StartedAt, &r.FinishedAt, &r.PausedAt, &r.PausedMS, &r.TimeoutSeconds, &r.Source, &r.ActorID,
		&r.RootRunID, &r.ParentRunID, &r.RetryNumber, &r.Archived, &r.CleanedAt, &r.Reason)
	return r, taskLookup(err)
}

func (r *Repository) run(ctx context.Context, id string) (Run, error) {
	item, err := scanRun(r.db.QueryRow(ctx, `SELECT `+runColumns+` FROM operations.runs r WHERE r.id=$1`, id))
	if err != nil {
		return item, err
	}
	c := &item.Counts
	if err := r.db.QueryRow(ctx, `SELECT `+countColumns+` FROM operations.targets WHERE run_id=$1`, id).
		Scan(&c.Total, &c.Succeeded, &c.Failed, &c.Skipped, &c.Cancelled, &c.Pending, &c.Usable); err != nil {
		return item, err
	}
	end := r.now()
	if item.FinishedAt != nil {
		end = *item.FinishedAt
	}
	item.ElapsedMS = taskprotocol.Elapsed(end, item.StartedAt, item.PausedAt, time.Duration(item.PausedMS)*time.Millisecond).Milliseconds()
	start := end
	if item.StartedAt != nil {
		start = *item.StartedAt
	}
	item.QueueMS = max(0, start.Sub(item.CreatedAt).Milliseconds())
	item.AllowedActions = item.State.Actions(item.Archived)
	if item.CleanedAt != nil {
		item.AllowedActions = []string{"unarchive"}
	}
	return item, nil
}

func normalizeQuery(q Query) (Query, error) {
	if q.Page == 0 {
		q.Page = 1
	}
	if q.PageSize == 0 {
		q.PageSize = 20
	}
	if q.Page < 1 || q.Page > 1000000 || q.PageSize < 1 || q.PageSize > 100 {
		return q, invalid(errors.New("invalid pagination"))
	}
	for _, value := range []string{q.From, q.To} {
		if value != "" {
			if _, err := time.Parse(time.RFC3339, value); err != nil {
				return q, invalid(errors.New("invalid time filter"))
			}
		}
	}
	if q.Schedule != "" {
		if _, err := uuid.Parse(q.Schedule); err != nil {
			return q, invalid(errors.New("invalid schedule filter"))
		}
	}
	return q, nil
}

func (r *Repository) Runs(ctx context.Context, input Query) (Page[Run], error) {
	q, err := normalizeQuery(input)
	page := Page[Run]{Items: []Run{}, Page: q.Page, PageSize: q.PageSize}
	if err != nil {
		return page, err
	}
	where := ` FROM operations.runs WHERE ($1='' OR kind=$1) AND ($2='' OR state=$2)
		AND ($3='' OR source=$3) AND ($4='' OR schedule_id=NULLIF($4,'')::uuid)
		AND ($5='' OR created_at>=NULLIF($5,'')::timestamptz) AND ($6='' OR created_at<=NULLIF($6,'')::timestamptz)
		AND ($7='' OR archived=NULLIF($7,'')::boolean)`
	args := []any{q.Kind, q.State, q.Source, q.Schedule, q.From, q.To, q.Archived}
	if err := r.db.QueryRow(ctx, `SELECT count(*)`+where, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	args = append(args, q.PageSize, (q.Page-1)*q.PageSize)
	rows, err := r.db.Query(ctx, `SELECT id::text`+where+` ORDER BY created_at DESC,id DESC LIMIT $8 OFFSET $9`, args...)
	if err != nil {
		return page, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return page, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return page, err
	}
	for _, id := range ids {
		item, err := r.run(ctx, id)
		if err != nil {
			return page, err
		}
		page.Items = append(page.Items, item)
	}
	return page, nil
}

const targetColumns = `id::text,run_id::text,site_id::text,feed_id::text,claim_id::text,kind,address,generation,
	access_scope,state,round_id::text,sample_started_at,reason,usable,recoveries,result`

func scanTarget(row pgx.Row) (Target, error) {
	var t Target
	err := row.Scan(&t.ID, &t.RunID, &t.SiteID, &t.FeedID, &t.ClaimID, &t.Kind, &t.Address, &t.Generation,
		&t.AccessScope, &t.State, &t.RoundID, &t.SampleStartedAt, &t.Reason, &t.Usable, &t.Recoveries, &t.Result)
	return t, err
}

func (r *Repository) Targets(ctx context.Context, id string, input Query) (Page[Target], error) {
	q, err := normalizeQuery(input)
	page := Page[Target]{Items: []Target{}, Page: q.Page, PageSize: q.PageSize}
	if err != nil {
		return page, err
	}
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM operations.targets WHERE run_id=$1 AND ($2='' OR state=$2)`, id, q.State).Scan(&page.Total); err != nil {
		return page, err
	}
	rows, err := r.db.Query(ctx, `SELECT `+targetColumns+` FROM operations.targets WHERE run_id=$1
		AND ($2='' OR state=$2) ORDER BY id LIMIT $3 OFFSET $4`, id, q.State, q.PageSize, (q.Page-1)*q.PageSize)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		t, err := scanTarget(rows)
		if err != nil {
			return page, err
		}
		t.Address = safeAddress(t.Address)
		page.Items = append(page.Items, t)
	}
	return page, rows.Err()
}

func safeAddress(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	u.RawQuery, u.Fragment, u.User = "", "", nil
	return u.String()
}
