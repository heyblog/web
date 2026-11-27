package taskcenter

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"errors"
	"time"

	"heyblog-api/internal/domain/proof"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type SigningKey struct {
	ID          string         `json:"id"`
	Platform    proof.Platform `json:"platform"`
	PublicKey   proof.JWK      `json:"public_key"`
	State       string         `json:"state"`
	CreatedAt   time.Time      `json:"created_at"`
	PublishedAt *time.Time     `json:"published_at"`
	RevokedAt   *time.Time     `json:"revoked_at"`
}
type SigningEvent struct {
	ID        string    `json:"id"`
	KeyID     *string   `json:"key_id"`
	ActorID   *string   `json:"actor_id"`
	Action    string    `json:"action"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Signing) Keys(ctx context.Context, public bool) ([]SigningKey, error) {
	rows, err := s.repository.db.Query(ctx, `SELECT id::text,platform,public_key,state,created_at,published_at,revoked_at FROM operations.signing_keys WHERE NOT $1 OR published_at IS NOT NULL ORDER BY created_at DESC`, public)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := []SigningKey{}
	for rows.Next() {
		var k SigningKey
		if err := rows.Scan(&k.ID, &k.Platform, &k.PublicKey, &k.State, &k.CreatedAt, &k.PublishedAt, &k.RevokedAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}
func (s *Signing) Generate(ctx context.Context, actor string, platform proof.Platform) (string, error) {
	if platform != proof.Cloudflare && platform != proof.EdgeOne {
		return "", invalid(errors.New("invalid platform"))
	}
	private, err := proof.GenerateKey()
	if err != nil {
		return "", err
	}
	id := uuid.NewString()
	ciphertext, err := proof.Encrypt(s.master, id, private)
	if err != nil {
		return "", err
	}
	public, err := proof.PublicKey(&private.PublicKey, id)
	if err != nil {
		return "", err
	}
	err = s.repository.transaction(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO operations.signing_keys(id,platform,public_key,encrypted_private_key) VALUES($1,$2,$3,$4)`, id, platform, public, ciphertext); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO operations.signing_events(key_id,actor_id,action) VALUES($1,$2,'generated')`, id, actor)
		return err
	})
	return id, err
}
func (s *Signing) Transition(ctx context.Context, actor, id, action string) error {
	return s.repository.transaction(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(73921409)`); err != nil {
			return err
		}
		var platform, state string
		if err := tx.QueryRow(ctx, `SELECT platform,state FROM operations.signing_keys WHERE id=$1 FOR UPDATE`, id).Scan(&platform, &state); err != nil {
			return err
		}
		switch action {
		case "publish":
			if state != "generated" {
				return conflict()
			}
			if _, err := tx.Exec(ctx, `UPDATE operations.signing_keys SET state='published',published_at=$2 WHERE id=$1`, id, s.repository.now()); err != nil {
				return err
			}
		case "activate":
			if state != "published" {
				return conflict()
			}
			if _, err := tx.Exec(ctx, `UPDATE operations.signing_keys SET state='retired' WHERE platform=$1 AND state='active'`, platform); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE operations.signing_keys SET state='active' WHERE id=$1`, id); err != nil {
				return err
			}
		case "revoke":
			if state == "revoked" {
				return conflict()
			}
			if _, err := tx.Exec(ctx, `UPDATE operations.signing_keys SET state='revoked',revoked_at=$2 WHERE id=$1`, id, s.repository.now()); err != nil {
				return err
			}
		default:
			return invalid(errors.New("invalid key action"))
		}
		_, err := tx.Exec(ctx, `INSERT INTO operations.signing_events(key_id,actor_id,action) VALUES($1,$2,$3)`, id, actor, action)
		return err
	})
}
func (s *Signing) Events(ctx context.Context) ([]SigningEvent, error) {
	rows, err := s.repository.db.Query(ctx, `SELECT id::text,key_id::text,actor_id::text,action,created_at FROM operations.signing_events ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []SigningEvent{}
	for rows.Next() {
		var e SigningEvent
		if err := rows.Scan(&e.ID, &e.KeyID, &e.ActorID, &e.Action, &e.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

type ProofCheck struct {
	Status  string        `json:"status"`
	Claims  *proof.Claims `json:"claims,omitempty"`
	Binding string        `json:"binding"`
}

func (s *Signing) Verify(ctx context.Context, token string) (ProofCheck, error) {
	header, _, err := proof.Inspect(token)
	if errors.Is(err, proof.ErrMissing) {
		return ProofCheck{Status: "missing"}, nil
	}
	if err != nil {
		return ProofCheck{Status: "invalid"}, nil
	}
	if _, err := uuid.Parse(header.KeyID); err != nil {
		return ProofCheck{Status: "invalid"}, nil
	}
	var key proof.JWK
	var state string
	var platform proof.Platform
	err = s.repository.db.QueryRow(ctx, `SELECT public_key,state,platform FROM operations.signing_keys WHERE id=$1 AND published_at IS NOT NULL`, header.KeyID).Scan(&key, &state, &platform)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProofCheck{Status: "invalid"}, nil
	}
	if err != nil {
		return ProofCheck{}, err
	}
	x, xerr := base64.RawURLEncoding.DecodeString(key.X)
	y, yerr := base64.RawURLEncoding.DecodeString(key.Y)
	if xerr != nil || yerr != nil || len(x) != 32 || len(y) != 32 {
		return ProofCheck{Status: "invalid"}, nil
	}
	public, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), append(append([]byte{4}, x...), y...))
	if err != nil {
		return ProofCheck{Status: "invalid"}, nil
	}
	claims, err := proof.Verify(token, proof.Verification{Key: public, KeyID: header.KeyID, Revoked: state == "revoked", Now: s.repository.now()})
	status := "valid"
	switch {
	case errors.Is(err, proof.ErrExpired):
		status = "expired"
	case errors.Is(err, proof.ErrRevoked):
		status = "revoked"
	case err != nil:
		return ProofCheck{Status: "invalid"}, nil
	}
	if claims.Platform != platform {
		return ProofCheck{Status: "invalid"}, nil
	}
	return ProofCheck{Status: status, Claims: &claims}, nil
}
