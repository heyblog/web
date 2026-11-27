package taskcenter

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) sweep(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `UPDATE operations.targets t SET state='skipped',reason='challenge_changed',
		finished_at=$1,lease_token=NULL,lease_until=NULL,worker_id=NULL WHERE kind='verification'
		AND state IN ('pending','running','retry_wait') AND NOT EXISTS(SELECT 1 FROM directory.site_claims c
		WHERE c.id=t.claim_id AND (c.status='PENDING' OR c.status='VERIFIED' AND
		 (COALESCE((t.progress->'CF'->>'valid')::boolean,false) OR COALESCE((t.progress->'EO'->>'valid')::boolean,false)))
		AND c.expires_at>$1
		AND c.token_hash=t.claim_snapshot->>'token_hash' AND c.user_id::text=t.claim_snapshot->>'user_id'
		AND c.address=t.claim_snapshot->>'address' AND c.method=t.claim_snapshot->>'method'
        AND c.email_change_id::text IS NOT DISTINCT FROM t.claim_snapshot->>'email_change_id'
        AND c.email_change_ownership_id::text IS NOT DISTINCT FROM t.claim_snapshot->>'email_change_ownership_id'
        AND (c.status='VERIFIED' OR c.email_change_id IS NULL OR EXISTS (
            SELECT 1 FROM identity.email_change_requests e JOIN identity.users u ON u.id=e.user_id
            WHERE e.id=c.email_change_id AND e.user_id=c.user_id AND e.auth_version=u.auth_version
            AND e.expires_at>clock_timestamp() AND u.access_status='ACTIVE'
            AND u.deleted_at IS NULL AND u.deletion_requested_at IS NULL)))`, r.now()); err != nil {
		return err
	}
	// Retire stale identities before accounting for outstanding authorizations.
	if _, err := tx.Exec(ctx, `UPDATE operations.targets t SET state='skipped',reason='target_changed',finished_at=$1,
		lease_token=NULL,lease_until=NULL,worker_id=NULL
		FROM directory.sites s WHERE s.id=t.site_id AND t.state IN ('pending','running','retry_wait') AND
		(s.monitoring_generation<>t.generation OR s.visibility<>'VISIBLE' OR
		 t.kind='rss' AND NOT EXISTS(SELECT 1 FROM directory.site_feeds f WHERE f.id=t.feed_id AND f.is_enabled
		   AND f.site_id=t.site_id AND t.address=CASE WHEN f.location_type='EXTERNAL' THEN f.external_url
		    ELSE s.scheme||'://'||s.normalized_host||f.url_ref END))`, r.now()); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE operations.request_statistics s SET unknown=true
		FROM operations.request_attempts a JOIN operations.targets t ON t.id=a.target_id
		WHERE s.attempt_id=a.id AND a.state IN ('authorized','sent')
		AND (t.finished_at IS NOT NULL OR t.state='running' AND t.lease_until<=$1)`, r.now()); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE operations.request_attempts a SET state='unknown'
		FROM operations.targets t WHERE a.target_id=t.id AND a.state IN ('authorized','sent')
		AND (t.finished_at IS NOT NULL OR t.state='running' AND t.lease_until<=$1)`, r.now()); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE operations.targets t SET
		state=CASE WHEN r.state='cancelling' THEN 'cancelled' WHEN recoveries>=3 THEN 'failed' ELSE 'pending' END,
		reason=CASE WHEN r.state='cancelling' THEN 'run_cancelled' WHEN recoveries>=3 THEN 'recovery_limit' ELSE 'lease_expired' END,
		recoveries=LEAST(recoveries+1,4),lease_token=NULL,lease_until=NULL,worker_id=NULL,
		finished_at=CASE WHEN r.state='cancelling' OR recoveries>=3 THEN $1 ELSE NULL END
		FROM operations.runs r WHERE t.run_id=r.id AND t.state='running' AND t.lease_until<=$1`, r.now()); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE operations.runs SET state='timed_out',reason='run_deadline',finished_at=$1
		WHERE state IN ('running','pausing','cancelling') AND started_at IS NOT NULL
		AND EXTRACT(EPOCH FROM ($1-started_at))*1000-paused_ms>=timeout_seconds::bigint*1000`, r.now()); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE operations.request_statistics s SET unknown=true
		FROM operations.request_attempts a JOIN operations.targets t ON t.id=a.target_id JOIN operations.runs r ON r.id=t.run_id
		WHERE s.attempt_id=a.id AND r.state='timed_out' AND a.state IN ('authorized','sent')`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE operations.targets t SET state='timed_out',reason='run_deadline',
		lease_token=NULL,lease_until=NULL,worker_id=NULL,finished_at=$1 FROM operations.runs r
		WHERE t.run_id=r.id AND r.state='timed_out' AND t.state IN ('pending','running','retry_wait')`, r.now()); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE operations.request_attempts a SET state='unknown'
		FROM operations.targets t WHERE a.target_id=t.id AND t.state='timed_out'
		AND a.state IN ('authorized','sent')`); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT id::text FROM operations.runs WHERE state IN ('queued','running','pausing','cancelling') ORDER BY id`)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if err := r.aggregate(ctx, tx, id); err != nil {
			return err
		}
	}
	return nil
}
