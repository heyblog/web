package taskcenter

import (
	"context"
	"errors"
	"time"

	"heyblog-api/internal/features/monitoring"

	"heyblog.local/edgerequest"
	"heyblog.local/taskprotocol"

	"github.com/jackc/pgx/v5"
)

type Execution struct{ repository *Repository }
type ExecutionResult = taskprotocol.ExecutionResult
type PathResult = taskprotocol.PathResult

func NewExecution(r *Repository) *Execution { return &Execution{repository: r} }

func allowFallback(result ExecutionResult) bool {
	if result.BusinessValid {
		return false
	}
	for _, path := range result.Paths {
		switch edgerequest.Kind(path.ErrorKind) {
		case edgerequest.Signature, edgerequest.Security, edgerequest.Cancelled:
			return false
		}
	}
	return true
}

func (e *Execution) Finish(ctx context.Context, id string, in taskprotocol.Finish) (ExecutionResult, error) {
	result := ExecutionResult{Paths: []PathResult{}}
	r := e.repository
	err := r.transaction(ctx, func(tx pgx.Tx) error {
		if err := taskLock(ctx, tx); err != nil {
			return err
		}
		if err := (&Repository{db: tx, now: r.now}).worker(ctx, in.WorkerInput, false); err != nil {
			return err
		}
		var finished bool
		var saved *ExecutionResult
		err := tx.QueryRow(ctx, `SELECT finished_at IS NOT NULL,result FROM operations.targets
			WHERE id=$1 AND worker_id=$2 AND lease_token=$3 FOR UPDATE`, id, in.InstanceID, in.Token).
			Scan(&finished, &saved)
		if err != nil {
			return taskLookup(err)
		}
		if finished {
			if saved != nil {
				result = *saved
			}
			return nil
		}
		if in.Reason == "cancelled" {
			var runID string
			err := tx.QueryRow(ctx, `UPDATE operations.targets t SET state='cancelled',reason='run_cancelled',finished_at=$4,
				lease_until=NULL FROM operations.runs r WHERE t.id=$1 AND t.worker_id=$2 AND t.lease_token=$3
				AND t.state='running' AND r.id=t.run_id AND r.state='cancelling' RETURNING t.run_id::text`,
				id, in.InstanceID, in.Token, r.now()).Scan(&runID)
			if err != nil {
				return taskLookup(err)
			}
			if _, err := tx.Exec(ctx, `UPDATE operations.request_statistics s SET unknown=true
				FROM operations.request_attempts a WHERE s.attempt_id=a.id AND a.target_id=$1
				AND a.state IN ('authorized','sent')`, id); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE operations.request_attempts SET state='unknown'
				WHERE target_id=$1 AND state IN ('authorized','sent')`, id); err != nil {
				return err
			}
			return r.aggregate(ctx, tx, runID)
		}
		l, err := (&Repository{db: tx, now: r.now}).loadLease(ctx, tx, id, in.Fence)
		if err != nil {
			return err
		}
		if len(l.Route) == 0 {
			return conflict()
		}
		expired := l.Target.Kind == "monitor" && l.Target.SampleStartedAt != nil && r.now().Sub(*l.Target.SampleStartedAt) > 5*time.Minute
		for _, platform := range l.Route {
			p := l.Progress[platform]
			if !p.Done {
				if expired || in.Reason == "interrupted" || p.Attempts > l.Configuration.Retries {
					p.Done = true
					p.Path.Platform, p.Path.Outcome, p.Path.ErrorKind = platform, "UNKNOWN", "interrupted"
					if expired {
						p.Path.ErrorKind = "round_expired"
					}
				} else if l.Target.Kind != "monitor" && !allowFallback(result) {
					break
				} else {
					return conflict()
				}
			}
			result.Paths = append(result.Paths, p.Path)
			result.BusinessValid = result.BusinessValid || p.Valid
			if taskprotocol.BusinessComplete(l.Target.Kind, p) {
				break
			}
		}
		result.Fallback = l.Target.Kind != "monitor" && len(result.Paths) > 1
		state, reason, usable := executionState(l.Target.Kind, result)
		if l.Target.Kind == "monitor" && l.Target.SampleStartedAt != nil {
			round := monitoring.Round{SiteID: l.Target.SiteID, Generation: l.Target.Generation, ID: l.Target.RoundID,
				StartedAt: *l.Target.SampleStartedAt, CN: monitoring.Unknown, GL: monitoring.Unknown}
			for _, p := range result.Paths {
				if p.Platform == "EO" {
					round.CN = monitoring.Outcome(p.Outcome)
				} else {
					round.GL = monitoring.Outcome(p.Outcome)
				}
			}
			if err := monitoring.RecordTx(ctx, tx, round); err != nil && !errors.Is(err, monitoring.ErrStaleGeneration) {
				return err
			}
		}
		_, err = tx.Exec(ctx, `UPDATE operations.targets SET state=$2,reason=$3,usable=$4,result=$5,finished_at=$6,lease_until=NULL
			WHERE id=$1`, id, state, reason, usable, result, r.now())
		if err != nil {
			return err
		}
		return r.aggregate(ctx, tx, l.Target.RunID)
	})
	return result, err
}

func executionState(kind string, result ExecutionResult) (string, string, bool) {
	complete, usable := true, result.BusinessValid
	for _, p := range result.Paths {
		if kind == "verification" && p.Outcome == "OK" {
			return "succeeded", p.ErrorKind, true
		}
		if p.Outcome == "UNKNOWN" {
			complete = false
		} else if kind == "monitor" {
			usable = true
		}
	}
	if kind == "monitor" && len(result.Paths) == 2 && complete || kind != "monitor" && result.BusinessValid {
		return "succeeded", "", true
	}
	if usable {
		return "partial_failed", "incomplete_execution", true
	}
	return "failed", "execution_failed", false
}
