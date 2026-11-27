package taskcenter

import (
	"context"
	"errors"
	"net/http"
	"time"

	"heyblog-api/internal/domain/proof"

	"heyblog.local/edgerequest"

	"github.com/jackc/pgx/v5"
)

type Signing struct {
	repository *Repository
	master     []byte
}

func NewSigning(r *Repository, master []byte) *Signing {
	return &Signing{repository: r, master: master}
}

type signingRequest struct {
	Platform        proof.Platform
	Method, URL, ID string
	Headers         http.Header
	Configuration   Configuration
	IssuedAt        time.Time
}

func (s *Signing) sign(ctx context.Context, tx pgx.Tx, in signingRequest) (http.Header, string, int, error) {
	var p Policy
	err := tx.QueryRow(ctx, `SELECT enabled,carrier,ttl_seconds,revision FROM operations.signing_policy WHERE id=1 FOR SHARE`).
		Scan(&p.Enabled, &p.Carrier, &p.TTLSeconds, &p.Revision)
	if err != nil {
		return nil, "", 0, err
	}
	headers := in.Headers.Clone()
	keyID := ""
	if in.Configuration.Signed {
		if !p.Enabled {
			return nil, "", 0, &edgerequest.Error{Kind: edgerequest.Signature, Cause: errors.New("signing is suspended")}
		}
		var encrypted []byte
		err := tx.QueryRow(ctx, `SELECT id::text,encrypted_private_key FROM operations.signing_keys
			WHERE platform=$1 AND state='active' FOR SHARE`, in.Platform).Scan(&keyID, &encrypted)
		if err != nil {
			return nil, "", 0, &edgerequest.Error{Kind: edgerequest.Signature, Cause: err}
		}
		key, err := proof.Decrypt(s.master, keyID, encrypted)
		if err != nil {
			return nil, "", 0, &edgerequest.Error{Kind: edgerequest.Signature, Cause: err}
		}
		ttl := min(in.Configuration.TTLSeconds, p.TTLSeconds)
		token, err := proof.Sign(key, keyID, proof.Claims{Method: in.Method, URL: in.URL, Platform: in.Platform,
			IssuedAt: in.IssuedAt.Unix(), ExpiresAt: in.IssuedAt.Add(time.Duration(ttl) * time.Second).Unix(), ID: in.ID})
		if err != nil {
			return nil, "", 0, &edgerequest.Error{Kind: edgerequest.Signature, Cause: err}
		}
		headers.Set("X-HeyBlog-Proof", token)
		if in.Configuration.Carrier == "both" && p.Carrier == "both" {
			headers.Set("User-Agent", headers.Get("User-Agent")+" HeyBlog-Proof/"+token)
		}
	}
	headers, err = edgerequest.SafeHeaders(headers)
	return headers, keyID, p.Revision, err
}

func (s *Signing) Policy(ctx context.Context) (Policy, error) {
	var p Policy
	err := s.repository.db.QueryRow(ctx, `SELECT enabled,carrier,ttl_seconds,revision FROM operations.signing_policy WHERE id=1`).
		Scan(&p.Enabled, &p.Carrier, &p.TTLSeconds, &p.Revision)
	return p, err
}

func (s *Signing) SetPolicy(ctx context.Context, actor string, p Policy) error {
	if p.Carrier != "both" && p.Carrier != "header" || p.TTLSeconds < 60 || p.TTLSeconds > 300 {
		return invalid(errors.New("invalid signing policy"))
	}
	return s.repository.transaction(ctx, func(tx pgx.Tx) error {
		if err := requireUpdated(tx.Exec(ctx, `UPDATE operations.signing_policy SET enabled=$1,carrier=$2,ttl_seconds=$3,
			revision=revision+1 WHERE id=1 AND revision=$4`, p.Enabled, p.Carrier, p.TTLSeconds, p.Revision)); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO operations.signing_events(actor_id,action) VALUES($1,'policy_updated')`, actor)
		return err
	})
}
