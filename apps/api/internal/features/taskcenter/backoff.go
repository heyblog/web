package taskcenter

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

func reservePlatform(ctx context.Context, tx pgx.Tx, platform, attempt string, now time.Time) (*time.Time, error) {
	var at, probeUntil *time.Time
	var probe *string
	err := tx.QueryRow(ctx, `SELECT retry_at,probe_id::text,probe_until FROM operations.platform_backoff WHERE platform=$1 FOR UPDATE`,
		platform).Scan(&at, &probe, &probeUntil)
	if err != nil || at == nil {
		return nil, err
	}
	if now.Before(*at) {
		return at, nil
	}
	if probe != nil && *probe != attempt && probeUntil != nil && now.Before(*probeUntil) {
		return probeUntil, nil
	}
	_, err = tx.Exec(ctx, `UPDATE operations.platform_backoff SET probe_id=$2,probe_until=$3 WHERE platform=$1`,
		platform, attempt, now.Add(60*time.Second))
	return nil, err
}

func updatePlatform(ctx context.Context, tx pgx.Tx, platform, attempt, reason string, now time.Time) error {
	if reason != "platform_rate_limited" && reason != "platform_exhausted" {
		// Only the reserved probe can reset shared response-driven backoff.
		_, err := tx.Exec(ctx, `UPDATE operations.platform_backoff SET retry_at=NULL,reason='',stage=0,probe_id=NULL,probe_until=NULL
			WHERE platform=$1 AND probe_id=$2`, platform, attempt)
		return err
	}
	var stage int
	if err := tx.QueryRow(ctx, `SELECT stage FROM operations.platform_backoff WHERE platform=$1 FOR UPDATE`, platform).Scan(&stage); err != nil {
		return err
	}
	delay := min(30*time.Minute, 30*time.Second*time.Duration(1<<stage))
	_, err := tx.Exec(ctx, `UPDATE operations.platform_backoff SET retry_at=$2,reason=$3,stage=LEAST(stage+1,6),
		probe_id=NULL,probe_until=NULL WHERE platform=$1`, platform, now.Add(delay), reason)
	return err
}
