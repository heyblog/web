package taskcenter

import (
	"context"

	"heyblog.local/taskprotocol"

	"github.com/jackc/pgx/v5"
)

type Send struct {
	taskprotocol.Fence
	AttemptID string `json:"attempt_id" format:"uuid"`
}

func (r *Repository) Sent(ctx context.Context, id string, in Send) error {
	return r.transaction(ctx, func(tx pgx.Tx) error {
		if err := taskLock(ctx, tx); err != nil {
			return err
		}
		if _, err := (&Repository{db: tx, now: r.now}).loadLease(ctx, tx, id, in.Fence); err != nil {
			return err
		}
		now := r.now()
		// A lost pre-send response cannot be replayed to obtain a second send.
		if err := requireUpdated(tx.Exec(ctx, `UPDATE operations.request_attempts a SET state='sent',sent_at=$5
			FROM operations.signing_policy p WHERE a.id=$1 AND a.target_id=$2 AND a.worker_id=$3 AND a.lease_token=$4
			AND a.state='authorized' AND a.expires_at>$5 AND p.id=1 AND p.revision=a.policy_revision
			AND (NOT a.signed OR p.enabled AND EXISTS(SELECT 1 FROM operations.signing_keys k WHERE k.id=a.key_id AND k.state='active'))`,
			in.AttemptID, id, in.InstanceID, in.Token, now)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE operations.targets SET sample_started_at=COALESCE(sample_started_at,$2)
			WHERE id=$1 AND kind='monitor'`, id, now); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE operations.request_statistics SET sent=true WHERE attempt_id=$1`, in.AttemptID)
		return err
	})
}
