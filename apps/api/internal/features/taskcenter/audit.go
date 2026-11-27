package taskcenter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

type Actor struct {
	ID     string
	System bool
}

type taskEvent struct {
	Run, Actor, Entity, Action, Outcome string
}

func recordEvent(ctx context.Context, tx pgx.Tx, e taskEvent) error {
	if e.Outcome == "" {
		e.Outcome = "accepted"
	}
	_, err := tx.Exec(ctx, `INSERT INTO operations.task_events(run_id,actor_id,entity_id,action,outcome)
		VALUES(NULLIF($1,'')::uuid,NULLIF($2,'')::uuid,NULLIF($3,'')::uuid,$4,$5)`,
		e.Run, e.Actor, e.Entity, e.Action, e.Outcome)
	return err
}

func payloadDigest(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func idempotent[T any](ctx context.Context, tx pgx.Tx, request mutation[T], execute func() (T, error)) (T, error) {
	var empty T
	if len(request.Key) < 1 || len(request.Key) > 120 || strings.ContainsAny(request.Key, "\r\n\t") {
		return empty, invalid(errors.New("Idempotency-Key is required"))
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,73921411))`, request.Scope+":"+request.Key); err != nil {
		return empty, err
	}
	var previous string
	var receipt T
	err := tx.QueryRow(ctx, `SELECT digest,result FROM operations.task_idempotency WHERE scope=$1 AND key=$2`, request.Scope, request.Key).Scan(&previous, &receipt)
	if err == nil {
		if previous != request.Digest {
			return empty, conflict()
		}
		return receipt, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return empty, err
	}
	receipt, err = execute()
	if err != nil {
		return empty, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO operations.task_idempotency(scope,key,digest,result) VALUES($1,$2,$3,$4)`,
		request.Scope, request.Key, request.Digest, receipt)
	return receipt, err
}

type mutation[T any] struct {
	Scope, Key, Digest string
}
