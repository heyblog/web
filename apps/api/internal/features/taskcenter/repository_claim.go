package taskcenter

import (
	"context"
	"net/http"
	"time"

	"heyblog.local/taskprotocol"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) Claim(ctx context.Context, in taskprotocol.WorkerInput) (*taskprotocol.Work, error) {
	var work *taskprotocol.Work
	err := r.transaction(ctx, func(tx pgx.Tx) error {
		scoped := &Repository{db: tx, now: r.now}
		if err := scoped.worker(ctx, in, true); err != nil {
			return err
		}
		if err := taskLock(ctx, tx); err != nil {
			return err
		}
		if err := r.sweep(ctx, tx); err != nil {
			return err
		}
		var id string
		err := tx.QueryRow(ctx, `SELECT t.id::text FROM operations.targets t JOIN operations.runs r ON r.id=t.run_id
			WHERE t.state IN ('pending','retry_wait') AND t.available_at<=$1 AND r.state IN ('queued','running')
			AND NOT EXISTS(SELECT 1 FROM operations.targets a WHERE a.site_id=t.site_id AND a.kind=t.kind AND a.state='running')
			AND (SELECT count(*) FROM operations.targets a WHERE a.run_id=t.run_id AND a.state='running')<
			 COALESCE((r.snapshot->>'concurrency')::int,1)
			AND (SELECT count(*) FROM operations.targets a WHERE a.worker_id=$2 AND a.state='running')<16
			ORDER BY r.created_at,t.id FOR UPDATE OF t,r SKIP LOCKED LIMIT 1`, r.now(), in.InstanceID).Scan(&id)
		if noRows(err) {
			return nil
		}
		if err != nil {
			return err
		}
		var token, runID string
		if err := tx.QueryRow(ctx, `UPDATE operations.targets SET state='running',worker_id=$2,lease_token=uuidv7(),lease_until=$3
			WHERE id=$1 RETURNING lease_token::text,run_id::text`, id, in.InstanceID, r.now().Add(120*time.Second)).Scan(&token, &runID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE operations.runs SET state='running',started_at=COALESCE(started_at,$2) WHERE id=$1`, runID, r.now()); err != nil {
			return err
		}
		fence := taskprotocol.Fence{WorkerInput: in, Token: token}
		l, err := scoped.loadLease(ctx, tx, id, fence)
		if err != nil {
			return err
		}
		route, err := scoped.platforms(ctx, tx, l)
		if err != nil {
			return err
		}
		headers := http.Header{}
		if l.Target.Kind == "rss" && l.Target.FeedID != nil {
			if err := (&Execution{repository: scoped}).feedHeaders(ctx, *l.Target.FeedID, headers); err != nil {
				return err
			}
		}
		w := taskprotocol.Work{ID: id, RunID: runID, Kind: l.Target.Kind, Address: l.Target.Address,
			Token: token, LeaseUntil: *l.Target.LeaseUntil, SampleStartedAt: l.Target.SampleStartedAt,
			Configuration: l.Configuration, Route: route, Progress: l.Progress, Headers: map[string]string{}, MaxBytes: 1 << 20}
		for k := range headers {
			w.Headers[k] = headers.Get(k)
		}
		if l.Target.Kind == "verification" {
			w.SameOrigin = true
			var method string
			if err := tx.QueryRow(ctx, `SELECT claim_snapshot->>'method' FROM operations.targets WHERE id=$1`, id).Scan(&method); err != nil {
				return err
			}
			if method == "FILE" {
				w.MaxBytes = 512
			}
		}
		work = &w
		return nil
	})
	return work, err
}

func (r *Repository) loadLease(ctx context.Context, tx pgx.Tx, id string, f taskprotocol.Fence) (Lease, error) {
	var l Lease
	t := &l.Target
	if err := r.worker(ctx, f.WorkerInput, false); err != nil {
		return l, err
	}
	err := tx.QueryRow(ctx, `SELECT t.id::text,t.run_id::text,t.site_id::text,t.feed_id::text,t.claim_id::text,t.kind,
		t.address,t.generation,t.access_scope,t.state,t.lease_token::text,t.lease_until,t.round_id::text,t.sample_started_at,
		t.worker_id::text,r.snapshot,r.started_at,t.progress-'route',COALESCE(t.progress->'route','[]')
		FROM operations.targets t JOIN operations.runs r ON r.id=t.run_id
		WHERE t.id=$1 AND t.worker_id=$2 AND t.lease_token=$3 AND t.state='running' AND t.lease_until>$4
		AND r.state IN ('running','pausing') AND r.cleaned_at IS NULL FOR UPDATE OF r,t`,
		id, f.InstanceID, f.Token, r.now()).Scan(&t.ID, &t.RunID, &t.SiteID, &t.FeedID, &t.ClaimID, &t.Kind,
		&t.Address, &t.Generation, &t.AccessScope, &t.State, &t.LeaseToken, &t.LeaseUntil, &t.RoundID, &t.SampleStartedAt,
		&t.WorkerID, &l.Configuration, &l.StartedAt, &l.Progress, &l.Route)
	if err != nil {
		return l, taskLookup(err)
	}
	// Decisions lock the applicant before the site. Follow that order when
	// a worker result and the claim decision share this transaction.
	if t.Kind == "verification" {
		var applicant string
		if err := tx.QueryRow(ctx, `SELECT u.id::text FROM identity.users u
            JOIN directory.site_claims c ON c.user_id=u.id
            WHERE c.id=$1 AND u.deleted_at IS NULL AND u.access_status='ACTIVE' FOR UPDATE OF u`,
			t.ClaimID).Scan(&applicant); err != nil {
			if noRows(err) {
				return l, errStaleTarget
			}
			return l, err
		}
	}
	var valid bool
	if err := tx.QueryRow(ctx, `SELECT generation=monitoring_generation AND visibility='VISIBLE'
		FROM directory.sites s JOIN operations.targets t ON t.site_id=s.id WHERE t.id=$1 FOR UPDATE OF s`, id).Scan(&valid); err != nil {
		return l, err
	}
	if !valid {
		return l, errStaleTarget
	}
	if t.Kind == "rss" {
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM directory.site_feeds f JOIN directory.sites s ON s.id=f.site_id
			WHERE f.id=$1 AND f.site_id=$2 AND f.is_enabled AND $3=CASE WHEN f.location_type='EXTERNAL'
			THEN f.external_url ELSE s.scheme||'://'||s.normalized_host||f.url_ref END)`,
			t.FeedID, t.SiteID, t.Address).Scan(&valid); err != nil {
			return l, err
		}
		if !valid {
			return l, errStaleTarget
		}
	}
	if t.Kind == "verification" {
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM directory.site_claims c JOIN operations.targets t ON t.claim_id=c.id
			WHERE t.id=$1 AND (c.status='PENDING' OR c.status='VERIFIED' AND
			 (COALESCE((t.progress->'CF'->>'valid')::boolean,false) OR COALESCE((t.progress->'EO'->>'valid')::boolean,false)))
			AND c.expires_at>$2 AND c.token_hash=t.claim_snapshot->>'token_hash'
			AND c.user_id::text=t.claim_snapshot->>'user_id' AND c.address=t.claim_snapshot->>'address'
            AND c.method=t.claim_snapshot->>'method'
            AND c.email_change_id::text IS NOT DISTINCT FROM t.claim_snapshot->>'email_change_id'
            AND c.email_change_ownership_id::text IS NOT DISTINCT FROM t.claim_snapshot->>'email_change_ownership_id')`, id, r.now()).Scan(&valid); err != nil {
			return l, err
		}
		if !valid {
			return l, errStaleTarget
		}
	}
	return l, nil
}

func (r *Repository) platforms(ctx context.Context, tx pgx.Tx, l Lease) ([]string, error) {
	if len(l.Route) > 0 {
		return l.Route, nil
	}
	route := []string{"EO", "CF"}
	if l.Target.Kind != "monitor" {
		var scope *string
		if err := tx.QueryRow(ctx, `SELECT access_scope FROM directory.sites WHERE id=$1`, l.Target.SiteID).Scan(&scope); err != nil {
			return nil, err
		}
		switch {
		case scope != nil && *scope == "CN":
			route = []string{"EO"}
		case scope != nil && *scope == "GL":
			route = []string{"CF"}
		default:
			var first bool
			if err := tx.QueryRow(ctx, `INSERT INTO operations.route_state(site_id,operation,next_cf) VALUES($1,$2,false)
				ON CONFLICT(site_id,operation) DO UPDATE SET next_cf=NOT operations.route_state.next_cf RETURNING NOT next_cf`,
				l.Target.SiteID, l.Target.Kind).Scan(&first); err != nil {
				return nil, err
			}
			if first {
				route = []string{"CF", "EO"}
			}
		}
	}
	_, err := tx.Exec(ctx, `UPDATE operations.targets SET progress=jsonb_set(progress,'{route}',$2) WHERE id=$1`, l.Target.ID, route)
	return route, err
}
