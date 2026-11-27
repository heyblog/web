package taskcenter

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type newRun struct {
	Schedule      *string
	Kind          string
	Configuration Configuration
	At            time.Time
	Source        string
	Actor         Actor
	Parent        *Run
	Selection     string
}

func (r *Repository) createRun(ctx context.Context, tx pgx.Tx, input newRun) (string, error) {
	if err := taskLock(ctx, tx); err != nil {
		return "", err
	}
	if input.Configuration.RunTimeoutSeconds == 0 {
		input.Configuration.RunTimeoutSeconds = 86400
	}
	id := uuid.NewString()
	root, sequence := id, 0
	var parent *string
	if input.Parent != nil {
		parent = &input.Parent.ID
		root = input.Parent.ID
		if input.Parent.RootRunID != nil {
			root = *input.Parent.RootRunID
		}
		if err := tx.QueryRow(ctx, `SELECT COALESCE(max(retry_number),0)+1 FROM operations.runs WHERE root_run_id=$1`, root).Scan(&sequence); err != nil {
			return "", err
		}
	}
	_, err := tx.Exec(ctx, `INSERT INTO operations.runs(id,schedule_id,kind,snapshot,scheduled_at,created_at,
		source,actor_id,request_config_id,root_run_id,parent_run_id,retry_number,timeout_seconds)
		VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::uuid,NULLIF($9,'')::uuid,$10,$11,$12,$13)`,
		id, input.Schedule, input.Kind, input.Configuration, input.At, r.now(), input.Source, input.Actor.ID,
		input.Configuration.RequestConfigID, root, parent, sequence, input.Configuration.RunTimeoutSeconds)
	if err != nil {
		return "", err
	}
	if parent != nil {
		_, err = tx.Exec(ctx, `INSERT INTO operations.targets(run_id,site_id,feed_id,kind,address,generation,access_scope)
			SELECT $1,t.site_id,t.feed_id,t.kind,s.scheme||'://'||s.normalized_host||s.base_path,s.monitoring_generation,s.access_scope
			FROM operations.targets t JOIN directory.sites s ON s.id=t.site_id WHERE t.run_id=$2 AND
			($3='all' OR $3='failed' AND t.state IN ('failed','partial_failed','timed_out')
			 OR $3='unfinished' AND t.state IN ('pending','retry_wait','running','cancelled','timed_out','failed','partial_failed'))`,
			id, *parent, input.Selection)
		// Feed identities remain fixed but current addresses are revalidated on claim.
		if err == nil && input.Kind == "rss" {
			_, err = tx.Exec(ctx, `UPDATE operations.targets t SET address=CASE WHEN f.location_type='EXTERNAL' THEN f.external_url
				ELSE s.scheme||'://'||s.normalized_host||f.url_ref END FROM directory.site_feeds f,directory.sites s
				WHERE t.run_id=$1 AND f.id=t.feed_id AND s.id=t.site_id`, id)
		}
	} else if input.Kind != "verification" {
		var ids []string
		if input.Configuration.Scope == "selected" {
			ids = input.Configuration.SiteIDs
		}
		if input.Kind == "monitor" {
			_, err = tx.Exec(ctx, `INSERT INTO operations.targets(run_id,site_id,kind,address,generation,access_scope)
				SELECT $1,id,'monitor',scheme||'://'||normalized_host||base_path,monitoring_generation,access_scope
				FROM directory.sites WHERE visibility='VISIBLE' AND ($2::uuid[] IS NULL OR id=ANY($2))`, id, ids)
		} else {
			_, err = tx.Exec(ctx, `INSERT INTO operations.targets(run_id,site_id,feed_id,kind,address,generation,access_scope)
				SELECT $1,s.id,f.id,'rss',CASE WHEN f.location_type='EXTERNAL' THEN f.external_url
				ELSE s.scheme||'://'||s.normalized_host||f.url_ref END,s.monitoring_generation,s.access_scope
				FROM directory.sites s JOIN directory.site_feeds f ON f.site_id=s.id WHERE s.visibility='VISIBLE'
				AND f.is_enabled AND ($2::uuid[] IS NULL OR s.id=ANY($2)) AND ($3='all' OR f.is_default)`,
				id, ids, input.Configuration.FeedMode)
		}
	}
	if err != nil {
		return "", err
	}
	if input.Kind != "verification" {
		if err := r.aggregate(ctx, tx, id); err != nil {
			return "", err
		}
	}
	err = recordEvent(ctx, tx, taskEvent{Run: id, Actor: input.Actor.ID, Action: "run_created"})
	return id, err
}

func (r *Repository) Manual(ctx context.Context, actor Actor, scheduleID string) (string, error) {
	var id string
	err := r.transaction(ctx, func(tx pgx.Tx) error {
		s, err := scanSchedule(tx.QueryRow(ctx, `SELECT `+scheduleColumns+` FROM operations.schedules WHERE id=$1 AND NOT archived FOR UPDATE`, scheduleID))
		if err != nil {
			return taskLookup(err)
		}
		p, err := loadConfiguration(ctx, tx, s.Configuration.RequestConfigID, s.Kind)
		if err != nil {
			return err
		}
		if !p.Signed && !actor.System {
			return unsignedDenied()
		}
		s.Configuration = bindParameters(s.Configuration, p)
		id, err = r.createRun(ctx, tx, newRun{Schedule: &s.ID, Kind: s.Kind, Configuration: s.Configuration, At: r.now(), Source: "manual", Actor: actor})
		return err
	})
	return id, err
}

type ManualInput struct {
	Kind          string        `json:"kind" enum:"monitor,rss"`
	Configuration Configuration `json:"configuration"`
}

func (r *Repository) Create(ctx context.Context, actor Actor, input ManualInput) (string, error) {
	var id string
	err := r.transaction(ctx, func(tx pgx.Tx) error {
		p, err := loadConfiguration(ctx, tx, input.Configuration.RequestConfigID, input.Kind)
		if err != nil {
			return err
		}
		if !p.Signed && !actor.System {
			return unsignedDenied()
		}
		c := bindParameters(input.Configuration, p)
		check := c
		check.Mode, check.IntervalSeconds, check.Timezone = "interval", 3600, "UTC"
		if _, err := (ScheduleInput{Name: "manual", Kind: input.Kind, Configuration: check}).Validate(r.now()); err != nil {
			return invalid(err)
		}
		if c.RunTimeoutSeconds < 60 || c.RunTimeoutSeconds > 604800 || (c.FeedMode != "all" && c.FeedMode != "default") {
			return invalid(errors.New("invalid run timeout or feed selection"))
		}
		id, err = r.createRun(ctx, tx, newRun{Kind: input.Kind, Configuration: c, At: r.now(), Source: "manual", Actor: actor})
		return err
	})
	return id, err
}
