package taskcenter

import (
	"context"
	"errors"
	"slices"

	"heyblog.local/taskprotocol"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) aggregate(ctx context.Context, tx pgx.Tx, id string) error {
	item, err := scanRun(tx.QueryRow(ctx, `SELECT `+runColumns+` FROM operations.runs r WHERE id=$1 FOR UPDATE`, id))
	if err != nil || item.State.Terminal() {
		return err
	}
	var c taskprotocol.TaskCounts
	var active int
	if err := tx.QueryRow(ctx, `SELECT `+countColumns+`,count(*) FILTER(WHERE state='running')
		FROM operations.targets WHERE run_id=$1`, id).Scan(&c.Total, &c.Succeeded, &c.Failed, &c.Skipped, &c.Cancelled, &c.Pending, &c.Usable, &active); err != nil {
		return err
	}
	next := item.State
	reason := item.Reason
	switch item.State {
	case taskprotocol.Cancelling:
		if active == 0 {
			next = taskprotocol.Cancelled
		}
	case taskprotocol.Pausing:
		if active == 0 {
			next = taskprotocol.Paused
		}
	case taskprotocol.Queued, taskprotocol.Running:
		if c.Pending == 0 {
			next = c.State()
			if c.Total == 0 {
				reason = "no_eligible_targets"
			}
		}
	case taskprotocol.Paused:
	}
	if next == item.State {
		return nil
	}
	_, err = tx.Exec(ctx, `UPDATE operations.runs SET state=$2,reason=$3,
		finished_at=CASE WHEN $4 THEN $5 ELSE finished_at END,
		paused_at=CASE WHEN $2='paused' THEN $5 ELSE paused_at END WHERE id=$1`,
		id, next, reason, next.Terminal(), r.now())
	if err != nil {
		return err
	}
	return recordEvent(ctx, tx, taskEvent{Run: id, Action: "run_" + string(next)})
}

func (r *Repository) Control(ctx context.Context, actor Actor, id, action string) (Run, error) {
	err := r.transaction(ctx, func(tx pgx.Tx) error {
		if err := taskLock(ctx, tx); err != nil {
			return err
		}
		item, err := scanRun(tx.QueryRow(ctx, `SELECT `+runColumns+` FROM operations.runs r WHERE id=$1 FOR UPDATE`, id))
		if err != nil {
			return taskLookup(err)
		}
		state := item.State
		switch action {
		case "pause":
			switch state {
			case taskprotocol.Paused, taskprotocol.Pausing:
				return nil
			case taskprotocol.Queued, taskprotocol.Running:
				state = taskprotocol.Pausing
			default:
				return conflict().WithCurrentState(string(state))
			}
		case "resume":
			switch state {
			case taskprotocol.Running, taskprotocol.Queued:
				return nil
			case taskprotocol.Paused:
				state = taskprotocol.Running
			default:
				return conflict().WithCurrentState(string(state))
			}
		case "cancel":
			if state == taskprotocol.Cancelled || state == taskprotocol.Cancelling {
				return nil
			}
			if state.Terminal() {
				return conflict().WithCurrentState(string(state))
			}
			state = taskprotocol.Cancelling
			if _, err := tx.Exec(ctx, `UPDATE operations.targets SET state='cancelled',reason='run_cancelled',finished_at=$2
				WHERE run_id=$1 AND state IN ('pending','retry_wait')`, id, r.now()); err != nil {
				return err
			}
		case "archive", "unarchive":
			if !state.Terminal() {
				return conflict().WithCurrentState(string(state))
			}
			if _, err := tx.Exec(ctx, `UPDATE operations.runs SET archived=$2 WHERE id=$1`, id, action == "archive"); err != nil {
				return err
			}
		default:
			return invalid(errors.New("unsupported task action"))
		}
		if _, err := tx.Exec(ctx, `UPDATE operations.runs SET state=$2,
			paused_ms=paused_ms+CASE WHEN started_at IS NOT NULL AND paused_at IS NOT NULL AND $3='resume'
				THEN GREATEST(0,EXTRACT(EPOCH FROM ($4-paused_at))*1000)::bigint ELSE 0 END,
			paused_at=CASE WHEN $3='resume' THEN NULL ELSE paused_at END WHERE id=$1`, id, state, action, r.now()); err != nil {
			return err
		}
		if err := recordEvent(ctx, tx, taskEvent{Run: id, Actor: actor.ID, Action: action}); err != nil {
			return err
		}
		return r.aggregate(ctx, tx, id)
	})
	if err != nil {
		return Run{}, err
	}
	return r.run(ctx, id)
}

func (r *Repository) Retry(ctx context.Context, actor Actor, id, selection string) (string, error) {
	var created string
	err := r.transaction(ctx, func(tx pgx.Tx) error {
		if err := taskLock(ctx, tx); err != nil {
			return err
		}
		item, err := scanRun(tx.QueryRow(ctx, `SELECT `+runColumns+` FROM operations.runs r WHERE id=$1 FOR UPDATE`, id))
		if err != nil {
			return err
		}
		if !item.State.Terminal() || item.CleanedAt != nil || item.Kind == "verification" {
			return conflict()
		}
		if !slices.Contains([]string{"failed", "unfinished", "all"}, selection) {
			return invalid(errors.New("invalid retry selection"))
		}
		if !item.Snapshot.Signed && !actor.System {
			return unsignedDenied()
		}
		source := "retry"
		if selection == "all" {
			source = "rerun"
		}
		created, err = r.createRun(ctx, tx, newRun{Kind: item.Kind, Configuration: item.Snapshot,
			At: r.now(), Source: source, Actor: actor, Parent: &item, Selection: selection})
		return err
	})
	return created, err
}

func (r *Repository) Cleanup(ctx context.Context, actor Actor, id, confirmation string) (Run, error) {
	if !actor.System {
		return Run{}, unsignedDenied()
	}
	if confirmation != id {
		return Run{}, invalid(errors.New("confirmation must match the run identity"))
	}
	err := r.transaction(ctx, func(tx pgx.Tx) error {
		if err := taskLock(ctx, tx); err != nil {
			return err
		}
		item, err := scanRun(tx.QueryRow(ctx, `SELECT `+runColumns+` FROM operations.runs r WHERE id=$1 FOR UPDATE`, id))
		if err != nil {
			return err
		}
		if !item.State.Terminal() || !item.Archived {
			return conflict()
		}
		if item.CleanedAt != nil {
			return nil
		}
		if _, err := tx.Exec(ctx, `UPDATE operations.targets SET progress='{}',result=NULL,claim_snapshot=NULL,challenge_digest=NULL,
			address='',lease_token=NULL,lease_until=NULL,worker_id=NULL WHERE run_id=$1`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE operations.request_attempts SET response_meta=NULL,url=''
			WHERE target_id IN (SELECT id FROM operations.targets WHERE run_id=$1)`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM operations.task_events WHERE run_id=$1 AND actor_id IS NULL`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE operations.runs SET snapshot='{}',cleaned_at=$2 WHERE id=$1`, id, r.now()); err != nil {
			return err
		}
		return recordEvent(ctx, tx, taskEvent{Run: id, Actor: actor.ID, Action: "cleanup"})
	})
	if err != nil {
		return Run{}, err
	}
	return r.run(ctx, id)
}
