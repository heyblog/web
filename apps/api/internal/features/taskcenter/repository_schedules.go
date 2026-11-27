package taskcenter

import (
	"context"
	"errors"
	"time"

	"heyblog.local/taskprotocol"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const scheduleColumns = `id::text,name,kind,enabled,configuration,revision,next_at,archived`

func scanSchedule(row pgx.Row) (Schedule, error) {
	var s Schedule
	err := row.Scan(&s.ID, &s.Name, &s.Kind, &s.Enabled, &s.Configuration, &s.Revision, &s.NextAt, &s.Archived)
	return s, err
}

func (r *Repository) Schedules(ctx context.Context) ([]Schedule, error) {
	rows, err := r.db.Query(ctx, `SELECT `+scheduleColumns+` FROM operations.schedules ORDER BY created_at,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Schedule{}
	for rows.Next() {
		s, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

func (r *Repository) Save(ctx context.Context, actor Actor, id string, input ScheduleInput) (Schedule, error) {
	var saved Schedule
	err := r.transaction(ctx, func(tx pgx.Tx) error {
		c, err := loadConfiguration(ctx, tx, input.Configuration.RequestConfigID, input.Kind)
		if err != nil {
			return err
		}
		if !c.Signed && !actor.System {
			return unsignedDenied()
		}
		if id != "" {
			var oldSigned bool
			if err := tx.QueryRow(ctx, `SELECT (c.parameters->>'signed')::boolean FROM operations.schedules s
				JOIN operations.request_configs c ON c.id=s.request_config_id WHERE s.id=$1 FOR UPDATE OF s`, id).Scan(&oldSigned); err != nil {
				return taskLookup(err)
			}
			if !oldSigned && !actor.System {
				return unsignedDenied()
			}
		}
		input.Configuration = bindParameters(input.Configuration, c)
		if input.Configuration.RunTimeoutSeconds == 0 {
			input.Configuration.RunTimeoutSeconds = 86400
		}
		if input.Configuration.RunTimeoutSeconds < 60 || input.Configuration.RunTimeoutSeconds > 604800 ||
			(input.Configuration.FeedMode != "all" && input.Configuration.FeedMode != "default") {
			return invalid(errors.New("invalid run timeout or feed selection"))
		}
		next, err := input.Validate(r.now())
		if err != nil {
			return invalid(err)
		}
		var nextAt *time.Time
		if input.Enabled && !input.Archived {
			nextAt = &next
		}
		if id == "" {
			id = uuid.NewString()
			// Creation always requires an explicit later enable operation.
			input.Enabled, nextAt = false, nil
			saved, err = scanSchedule(tx.QueryRow(ctx, `INSERT INTO operations.schedules
				(id,name,kind,enabled,configuration,next_at,request_config_id,archived)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING `+scheduleColumns,
				id, input.Name, input.Kind, input.Enabled, input.Configuration, nextAt, input.Configuration.RequestConfigID, input.Archived))
		} else {
			saved, err = scanSchedule(tx.QueryRow(ctx, `UPDATE operations.schedules SET name=$2,kind=$3,enabled=$4,configuration=$5,
				next_at=CASE WHEN enabled AND $4 AND NOT $8 AND next_at IS NOT NULL
				 AND configuration->'mode'=($5::jsonb)->'mode'
				 AND configuration->'interval_seconds'=($5::jsonb)->'interval_seconds'
				 AND configuration->'cron'=($5::jsonb)->'cron'
				 AND configuration->'timezone'=($5::jsonb)->'timezone'
				 THEN next_at ELSE $6 END,
				request_config_id=$7,archived=$8,revision=revision+1 WHERE id=$1 AND revision=$9 RETURNING `+scheduleColumns,
				id, input.Name, input.Kind, input.Enabled, input.Configuration, nextAt, input.Configuration.RequestConfigID, input.Archived, input.Revision))
		}
		if err != nil {
			return taskLookup(err)
		}
		return recordEvent(ctx, tx, taskEvent{Actor: actor.ID, Entity: id, Action: "schedule_saved"})
	})
	return saved, err
}

func (r *Repository) Due(ctx context.Context, worker taskprotocol.WorkerInput) (taskprotocol.Due, error) {
	result := taskprotocol.Due{ServerTime: r.now(), Schedules: []taskprotocol.DueSchedule{}}
	if err := r.worker(ctx, worker, true); err != nil {
		return result, err
	}
	rows, err := r.db.Query(ctx, `SELECT id::text,revision,next_at,configuration FROM operations.schedules
		WHERE enabled AND NOT archived AND next_at<=$1 ORDER BY next_at,id LIMIT 100`, result.ServerTime)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var s taskprotocol.DueSchedule
		if err := rows.Scan(&s.ID, &s.Revision, &s.At, &s.Configuration); err != nil {
			return result, err
		}
		result.Schedules = append(result.Schedules, s)
	}
	return result, rows.Err()
}

func (r *Repository) Dispatch(ctx context.Context, in taskprotocol.Dispatch) (accepted, error) {
	var receipt accepted
	err := r.transaction(ctx, func(tx pgx.Tx) error {
		scoped := &Repository{db: tx, now: r.now}
		if err := scoped.worker(ctx, in.WorkerInput, true); err != nil {
			return err
		}
		s, err := scanSchedule(tx.QueryRow(ctx, `SELECT `+scheduleColumns+` FROM operations.schedules WHERE id=$1 FOR UPDATE`, in.ID))
		if err != nil {
			return taskLookup(err)
		}
		if !s.Enabled || s.Archived || s.Revision != in.Revision || s.NextAt == nil || !s.NextAt.Equal(in.At) || in.At.After(r.now()) {
			return conflict()
		}
		// Recompute using server time: downtime produces at most one catch-up.
		next, err := s.Configuration.Next(r.now())
		if err != nil {
			return err
		}
		if in.NextAt.Before(r.now()) || in.NextAt.After(next.Add(time.Minute)) {
			return conflict()
		}
		var active bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM operations.runs WHERE schedule_id=$1
			AND state IN ('queued','running','pausing','paused','cancelling'))`, s.ID).Scan(&active); err != nil {
			return err
		}
		if active {
			if err := recordEvent(ctx, tx, taskEvent{Entity: s.ID, Action: "scheduled_overlap", Outcome: "skipped"}); err != nil {
				return err
			}
		} else {
			p, err := loadConfiguration(ctx, tx, s.Configuration.RequestConfigID, s.Kind)
			if err == nil {
				s.Configuration = bindParameters(s.Configuration, p)
				receipt.ID, err = scoped.createRun(ctx, tx, newRun{Schedule: &s.ID, Kind: s.Kind, Configuration: s.Configuration, At: in.At, Source: "scheduled"})
			}
			if err != nil {
				if err := recordEvent(ctx, tx, taskEvent{Entity: s.ID, Action: "scheduled_config_unavailable", Outcome: "skipped"}); err != nil {
					return err
				}
			}
		}
		_, err = tx.Exec(ctx, `UPDATE operations.schedules SET next_at=$2 WHERE id=$1`, s.ID, next)
		return err
	})
	return receipt, err
}
