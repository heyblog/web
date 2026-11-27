package taskcenter

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"time"

	"heyblog.local/edgerequest"
	"heyblog.local/taskprotocol"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Signing) Permit(ctx context.Context, id string, in taskprotocol.Authorization) (taskprotocol.Permit, error) {
	var permit taskprotocol.Permit
	r := s.repository
	err := r.transaction(ctx, func(tx pgx.Tx) error {
		if err := taskLock(ctx, tx); err != nil {
			return err
		}
		l, err := (&Repository{db: tx, now: r.now}).loadLease(ctx, tx, id, in.Fence)
		if err != nil {
			return err
		}
		now := r.now()
		if l.Target.Kind == "monitor" && l.Target.SampleStartedAt != nil && now.Sub(*l.Target.SampleStartedAt) > 5*time.Minute {
			return conflict()
		}
		attemptID, issued := uuid.NewString(), now
		var oldKey *string
		var oldRevision int
		var state, oldURL, oldPlatform, oldMethod, oldParent, oldWorker, oldToken string
		var oldRetry, oldHop int
		err = tx.QueryRow(ctx, `SELECT id::text,created_at,key_id::text,policy_revision,state,url,platform,method,
			logical_retry,hop,COALESCE(parent_id::text,''),COALESCE(worker_id::text,''),COALESCE(lease_token::text,'')
			FROM operations.request_attempts
			WHERE target_id=$1 AND request_key=$2 FOR UPDATE`, id, in.RequestKey).
			Scan(&attemptID, &issued, &oldKey, &oldRevision, &state, &oldURL, &oldPlatform, &oldMethod,
				&oldRetry, &oldHop, &oldParent, &oldWorker, &oldToken)
		existing := err == nil
		if err != nil && !noRows(err) {
			return err
		}
		if existing && (state != "authorized" || oldURL != in.URL || oldPlatform != in.Platform ||
			oldMethod != in.Method || oldRetry != in.Retry || oldHop != in.Hop || oldParent != in.ParentID ||
			oldWorker != in.InstanceID || oldToken != in.Token || now.Sub(issued) >= 60*time.Second) {
			return conflict()
		}
		if !existing {
			if err := validateHop(ctx, tx, l, in); err != nil {
				return err
			}
			if err := tx.QueryRow(ctx, `SELECT next_dispatch_at FROM operations.runs WHERE id=$1`, l.Target.RunID).Scan(&permit.RetryAt); err != nil {
				return err
			}
			if permit.RetryAt != nil && now.Before(*permit.RetryAt) {
				permit.Reason = "request_interval"
				if permit.RetryAt.Sub(now) > 5*time.Second {
					permit.Reason = "request_interval_deferred"
					_, err := tx.Exec(ctx, `UPDATE operations.targets SET available_at=$2,state='retry_wait',
						lease_token=NULL,lease_until=NULL,worker_id=NULL,reason='request_interval'
						WHERE id=$1`, id, *permit.RetryAt)
					return err
				}
				return nil
			}
			permit.RetryAt = nil
		}
		retryAt, err := reservePlatform(ctx, tx, in.Platform, attemptID, now)
		if err != nil {
			return err
		}
		if retryAt != nil {
			permit.RetryAt, permit.Reason = retryAt, "platform_backoff"
			_, err := tx.Exec(ctx, `UPDATE operations.targets SET available_at=$2,state='retry_wait',
				lease_token=NULL,lease_until=NULL,worker_id=NULL,reason='platform_backoff' WHERE id=$1`, id, *retryAt)
			return err
		}
		headers := make(http.Header)
		for name, value := range l.Configuration.Headers {
			headers.Set(name, value)
		}
		headers.Set("User-Agent", l.Configuration.UserAgent)
		if l.Target.Kind == "rss" && l.Target.FeedID != nil {
			if err := (&Execution{repository: &Repository{db: tx, now: r.now}}).feedHeaders(ctx, *l.Target.FeedID, headers); err != nil {
				return err
			}
		}
		var key string
		var revision int
		headers, key, revision, err = s.sign(ctx, tx, signingRequest{Platform: edgerequest.Provider(in.Platform),
			Method: in.Method, URL: in.URL, ID: attemptID, Headers: headers, Configuration: l.Configuration, IssuedAt: issued})
		if err != nil {
			return err
		}
		if existing && (oldRevision != revision || oldKey != nil && *oldKey != key || oldKey == nil && key != "") {
			return conflict()
		}
		if !existing {
			_, err := tx.Exec(ctx, `INSERT INTO operations.request_attempts(id,target_id,platform,signed,request_key,kind,
				url,method,logical_retry,hop,parent_id,key_id,policy_revision,state,created_at,expires_at,worker_id,lease_token)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NULLIF($11,'')::uuid,NULLIF($12,'')::uuid,$13,'authorized',$14,$15,$16,$17)`,
				attemptID, id, in.Platform, l.Configuration.Signed, in.RequestKey, l.Target.Kind, in.URL, in.Method,
				in.Retry, in.Hop, in.ParentID, key, revision, now, now.Add(60*time.Second), in.InstanceID, in.Token)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO operations.request_statistics(attempt_id,platform,kind,created_at,authorized,retries,redirects,fallback)
				VALUES($1,$2,$3,$4,true,$5,$6,$7)`, attemptID, in.Platform, l.Target.Kind, now, in.Retry, in.Hop,
				l.Target.Kind != "monitor" && len(l.Route) > 0 && in.Platform != l.Route[0]); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE operations.runs SET next_dispatch_at=$2 WHERE id=$1`, l.Target.RunID,
				now.Add(time.Duration(l.Configuration.IntervalMS)*time.Millisecond)); err != nil {
				return err
			}
			progress := l.Progress[in.Platform]
			progress.Pending = nil
			progress.Attempts = in.Retry + 1
			progress.Path.Platform, progress.Path.Outcome, progress.Path.ErrorKind = in.Platform, "UNKNOWN", "interrupted"
			if _, err := tx.Exec(ctx, `UPDATE operations.targets SET progress=jsonb_set(progress,ARRAY[$2],$3) WHERE id=$1`,
				id, in.Platform, progress); err != nil {
				return err
			}
		}
		permit.ID, permit.ExpiresAt = attemptID, issued.Add(60*time.Second)
		permit.Headers = map[string][]byte{}
		for name := range headers {
			permit.Headers[name] = []byte(headers.Get(name))
		}
		return nil
	})
	return permit, err
}

func validateHop(ctx context.Context, tx pgx.Tx, l Lease, in taskprotocol.Authorization) error {
	if in.Retry < 0 || in.Retry > l.Configuration.Retries || in.Hop < 0 || in.Hop > l.Configuration.MaxRedirects ||
		!slices.Contains(l.Route, in.Platform) || in.Method != "GET" {
		return conflict()
	}
	if p, exists := l.Progress[in.Platform]; exists && (p.Done || in.Hop == 0 && p.Attempts > in.Retry) {
		return conflict()
	}
	expected := l.Target.Address
	if in.Hop > 0 {
		var previous string
		var status int
		var location string
		err := tx.QueryRow(ctx, `SELECT url,(response_meta->>'status')::int,response_meta->>'location'
			FROM operations.request_attempts WHERE id=NULLIF($1,'')::uuid AND target_id=$2 AND platform=$3
			AND logical_retry=$4 AND hop=$5 AND accepted AND state='received'`,
			in.ParentID, l.Target.ID, in.Platform, in.Retry, in.Hop-1).Scan(&previous, &status, &location)
		if err != nil {
			return taskLookup(err)
		}
		if status != 301 && status != 302 && status != 303 && status != 307 && status != 308 {
			return conflict()
		}
		u, err := url.Parse(previous)
		if err != nil {
			return err
		}
		next, err := u.Parse(location)
		if err != nil {
			return invalid(errors.New("invalid redirect"))
		}
		next.Fragment = ""
		expected = next.String()
	}
	canonical, err := edgerequest.NormalizeURL(expected)
	if err != nil || canonical != in.URL {
		return conflict()
	}
	if l.Target.Kind == "verification" {
		original, err := url.Parse(l.Target.Address)
		if err != nil {
			return err
		}
		current, err := url.Parse(in.URL)
		if err != nil || current.Scheme != original.Scheme || current.Host != original.Host {
			return conflict()
		}
	}
	return nil
}
