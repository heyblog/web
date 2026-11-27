package taskcenter

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"heyblog.local/edgerequest"
	"heyblog.local/taskprotocol"

	"github.com/jackc/pgx/v5"
)

type attemptMeta struct {
	Status     int                     `json:"status"`
	Location   string                  `json:"location,omitempty"`
	ErrorKind  string                  `json:"error_kind"`
	Evaluation taskprotocol.Evaluation `json:"evaluation"`
	DurationMS int64                   `json:"duration_ms"`
	Node       map[string]string       `json:"node"`
}

func (e *Execution) Report(ctx context.Context, id string, in taskprotocol.Report) (taskprotocol.Evaluation, error) {
	var evaluation taskprotocol.Evaluation
	digest, err := payloadDigest(in)
	if err != nil {
		return evaluation, err
	}
	r := e.repository
	err = r.transaction(ctx, func(tx pgx.Tx) error {
		if err := taskLock(ctx, tx); err != nil {
			return err
		}
		var previous *string
		var meta *attemptMeta
		var platform, state string
		var retry, hop int
		if err := tx.QueryRow(ctx, `SELECT response_digest,response_meta,platform,state,logical_retry,hop
			FROM operations.request_attempts WHERE id=$1 AND target_id=$2 AND worker_id=$3 AND lease_token=$4 FOR UPDATE`,
			in.AttemptID, id, in.InstanceID, in.Token).Scan(&previous, &meta, &platform, &state, &retry, &hop); err != nil {
			return taskLookup(err)
		}
		if previous != nil {
			if *previous != digest {
				return conflict()
			}
			if meta == nil {
				// Detail cleanup is a permanent receipt, never a fresh submission.
				return conflict()
			}
			evaluation = meta.Evaluation
			return nil
		}
		l, err := (&Repository{db: tx, now: r.now}).loadLease(ctx, tx, id, in.Fence)
		if err != nil {
			return err
		}
		if state != "sent" || len(in.Body) > 1<<20 || in.DurationMS < 0 || in.DurationMS > 90000 || len(in.Headers) > 16 {
			return invalid(errors.New("invalid attempt result"))
		}
		headers := http.Header{}
		for name, value := range in.Headers {
			switch strings.ToLower(name) {
			case "location", "etag", "last-modified", "retry-after", "content-type":
				if len(value) > 8192 || strings.ContainsAny(string(value), "\r\n") {
					return invalid(errors.New("invalid response metadata"))
				}
				headers.Set(name, string(value))
			}
		}
		outcome, reason, valid := "UNKNOWN", in.ErrorKind, false
		redirect := in.ErrorKind == "" && isRedirect(in.Status) && hop < l.Configuration.MaxRedirects && headers.Get("Location") != ""
		if !redirect {
			switch {
			case in.ErrorKind == "target":
				outcome = "FAIL"
			case in.ErrorKind != "":
			case isRedirect(in.Status):
				outcome, reason = "FAIL", "incomplete_redirect"
			case in.Status >= 200 && in.Status < 400:
				switch l.Target.Kind {
				case "monitor":
					outcome, valid = "OK", true
				case "rss":
					scoped := &Execution{repository: &Repository{db: tx, now: r.now}}
					valid, err = scoped.storeFeed(ctx, l, edgerequest.Result{Status: in.Status, Headers: headers, Body: in.Body})
					if err != nil {
						return err
					}
					if valid {
						outcome = "OK"
					} else {
						reason = "invalid_feed"
					}
				case "verification":
					if len(in.Body) > int(verificationLimit(ctx, tx, id)) {
						return invalid(errors.New("verification response exceeds limit"))
					}
					valid, err = e.verifyClaim(ctx, tx, l, in.Status, in.Body)
					if err != nil {
						return err
					}
					// Proof mismatch is a completed business decision.
					outcome, reason = "OK", "proof_mismatch"
					if valid {
						reason = ""
					}
				}
			default:
				outcome, reason = "FAIL", "target_http"
			}
		}
		evaluation = taskprotocol.Evaluation{Valid: valid, Continue: redirect, Reason: reason}
		resultMeta := attemptMeta{Status: in.Status, Location: headers.Get("Location"), ErrorKind: in.ErrorKind,
			Evaluation: evaluation, DurationMS: in.DurationMS, Node: map[string]string{}}
		if platform == "CF" && len(in.Node["colo"]) <= 16 {
			resultMeta.Node["colo"] = in.Node["colo"]
		}
		progress := l.Progress[platform]
		progress.Attempts = max(progress.Attempts, retry+1)
		progress.Path.Platform, progress.Path.Outcome, progress.Path.HTTPStatus = platform, outcome, in.Status
		progress.Path.ErrorKind, progress.Path.Node = reason, resultMeta.Node
		progress.Path.Requests++
		progress.Path.DurationMS += in.DurationMS
		now := r.now()
		progress.Path.ObservedAt = &now
		if redirect {
			progress.Path.Redirects = append(progress.Path.Redirects, safeAddress(headers.Get("Location")))
			var address string
			if err := tx.QueryRow(ctx, `SELECT url FROM operations.request_attempts WHERE id=$1`, in.AttemptID).Scan(&address); err != nil {
				return err
			}
			base, parseErr := url.Parse(address)
			if parseErr != nil {
				return parseErr
			}
			next, parseErr := base.Parse(headers.Get("Location"))
			if parseErr != nil {
				return invalid(parseErr)
			}
			next.Fragment = ""
			progress.Pending = &taskprotocol.PendingHop{URL: next.String(), Retry: retry, Hop: hop + 1, ParentID: in.AttemptID}
		} else {
			progress.Pending = nil
			progress.Valid = valid
			// Business-invalid RSS can use a backup route, never a network retry.
			progress.Done = valid || l.Target.Kind == "verification" && outcome == "OK" || reason == "invalid_feed" ||
				retry >= l.Configuration.Retries || (in.ErrorKind != "target" && in.ErrorKind != "platform" && outcome != "FAIL")
		}
		evaluation.Progress = &progress
		resultMeta.Evaluation = evaluation
		if _, err := tx.Exec(ctx, `UPDATE operations.targets SET progress=jsonb_set(progress,ARRAY[$2],$3) WHERE id=$1`,
			id, platform, progress); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE operations.request_attempts SET state='received',received_at=$2,response_digest=$3,
			response_meta=$4,accepted=true WHERE id=$1`, in.AttemptID, now, digest, resultMeta); err != nil {
			return err
		}
		succeeded := redirect || outcome == "OK"
		if _, err := tx.Exec(ctx, `UPDATE operations.request_statistics SET received=true,succeeded=$2,failed=$3,
			duration_ms=$4,unknown=false WHERE attempt_id=$1`, in.AttemptID, succeeded,
			!succeeded, in.DurationMS); err != nil {
			return err
		}
		platformReason := ""
		if in.ErrorKind == "platform" && (in.Status == 429 || in.Status == 402) {
			platformReason = "platform_rate_limited"
		}
		if in.ErrorKind == "" || platformReason != "" {
			return updatePlatform(ctx, tx, platform, in.AttemptID, platformReason, now)
		}
		return nil
	})
	return evaluation, err
}

func isRedirect(status int) bool {
	switch status {
	case 301, 302, 303, 307, 308:
		return true
	default:
		return false
	}
}

func verificationLimit(ctx context.Context, tx pgx.Tx, id string) int64 {
	var method string
	if err := tx.QueryRow(ctx, `SELECT claim_snapshot->>'method' FROM operations.targets WHERE id=$1`, id).Scan(&method); err != nil {
		return 0
	}
	if method == "FILE" {
		return 512
	}
	return 1 << 20
}
