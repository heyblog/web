package taskcenter

import (
	"context"
	"errors"
	"time"

	"heyblog.local/taskprotocol"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) Register(ctx context.Context, in taskprotocol.Instance) (taskprotocol.Worker, error) {
	var result taskprotocol.Worker
	if in.Protocol != taskprotocol.ProtocolVersion || len(in.Version) < 1 || len(in.Version) > 120 {
		return result, conflict()
	}
	err := r.db.QueryRow(ctx, `INSERT INTO operations.workers(id,protocol,version,registered_at,heartbeat_at)
		VALUES($1,$2,$3,$4,$4) ON CONFLICT(id) DO UPDATE SET heartbeat_at=EXCLUDED.heartbeat_at
		WHERE operations.workers.protocol=EXCLUDED.protocol AND operations.workers.version=EXCLUDED.version
		RETURNING id::text,protocol,version,registered_at,heartbeat_at,draining`,
		in.InstanceID, in.Protocol, in.Version, r.now()).Scan(&result.InstanceID, &result.Protocol, &result.Version,
		&result.RegisteredAt, &result.HeartbeatAt, &result.Draining)
	result.Online = err == nil
	return result, taskLookup(err)
}

func (r *Repository) worker(ctx context.Context, in taskprotocol.WorkerInput, claiming bool) error {
	if in.Protocol != taskprotocol.ProtocolVersion {
		return conflict()
	}
	var valid bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM operations.workers WHERE id=$1 AND protocol=$2
		AND heartbeat_at>$3 AND (NOT $4 OR NOT draining))`,
		in.InstanceID, in.Protocol, r.now().Add(-time.Minute), claiming).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return conflict()
	}
	return nil
}

func (r *Repository) WorkerHeartbeat(ctx context.Context, in taskprotocol.WorkerInput) (taskprotocol.Worker, error) {
	if in.Protocol != taskprotocol.ProtocolVersion {
		return taskprotocol.Worker{}, conflict()
	}
	var item taskprotocol.Worker
	err := r.transaction(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `UPDATE operations.workers SET heartbeat_at=$3 WHERE id=$1 AND protocol=$2
			RETURNING id::text,protocol,version,registered_at,heartbeat_at,draining`, in.InstanceID, in.Protocol, r.now()).
			Scan(&item.InstanceID, &item.Protocol, &item.Version, &item.RegisteredAt, &item.HeartbeatAt, &item.Draining)
		if err != nil {
			return taskLookup(err)
		}
		if err := taskLock(ctx, tx); err != nil {
			return err
		}
		return r.sweep(ctx, tx)
	})
	item.Online = err == nil
	return item, err
}

func (r *Repository) Workers(ctx context.Context) ([]taskprotocol.Worker, error) {
	rows, err := r.db.Query(ctx, `SELECT w.id::text,w.protocol,w.version,w.registered_at,w.heartbeat_at,w.draining,
		(SELECT count(*) FROM operations.targets t WHERE t.worker_id=w.id AND t.state='running' AND lease_until>$1)
		FROM operations.workers w ORDER BY heartbeat_at DESC LIMIT 100`, r.now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []taskprotocol.Worker{}
	for rows.Next() {
		var item taskprotocol.Worker
		if err := rows.Scan(&item.InstanceID, &item.Protocol, &item.Version, &item.RegisteredAt, &item.HeartbeatAt, &item.Draining, &item.Active); err != nil {
			return nil, err
		}
		item.Online = item.HeartbeatAt.After(r.now().Add(-time.Minute))
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) Drain(ctx context.Context, actor Actor, id string, draining bool) error {
	if !actor.System {
		return unsignedDenied()
	}
	return r.transaction(ctx, func(tx pgx.Tx) error {
		if err := requireUpdated(tx.Exec(ctx, `UPDATE operations.workers SET draining=$2 WHERE id=$1`, id, draining)); err != nil {
			return err
		}
		action := "worker_resumed"
		if draining {
			action = "worker_draining"
		}
		return recordEvent(ctx, tx, taskEvent{Actor: actor.ID, Entity: id, Action: action})
	})
}

func (r *Repository) Heartbeat(ctx context.Context, id string, fence taskprotocol.Fence) (string, error) {
	var state string
	err := r.transaction(ctx, func(tx pgx.Tx) error {
		if err := (&Repository{db: tx, now: r.now}).worker(ctx, fence.WorkerInput, false); err != nil {
			return err
		}
		if err := taskLock(ctx, tx); err != nil {
			return err
		}
		err := tx.QueryRow(ctx, `SELECT r.state FROM operations.targets t JOIN operations.runs r ON r.id=t.run_id
			WHERE t.id=$1 AND t.worker_id=$2 AND t.lease_token=$3 AND t.state='running'
			AND t.lease_until>$4 FOR UPDATE OF t,r`, id, fence.InstanceID, fence.Token, r.now()).Scan(&state)
		if err != nil {
			return taskLookup(err)
		}
		if state != "running" && state != "pausing" && state != "cancelling" {
			return conflict()
		}
		if state == "cancelling" {
			return nil
		}
		return requireUpdated(tx.Exec(ctx, `UPDATE operations.targets SET lease_until=$2 WHERE id=$1`, id, r.now().Add(120*time.Second)))
	})
	return state, err
}

var errStaleTarget = errors.New("target identity is no longer valid")
