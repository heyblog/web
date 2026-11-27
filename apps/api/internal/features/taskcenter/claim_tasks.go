package taskcenter

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"

	"heyblog-api/internal/features/sitemanagement"

	"github.com/jackc/pgx/v5"
)

type claimSnapshot struct {
	UserID                 string `json:"user_id"`
	Address                string `json:"address"`
	Method                 string `json:"method"`
	TokenHash              string `json:"token_hash"`
	EmailChangeID          string `json:"email_change_id,omitempty"`
	EmailChangeOwnershipID string `json:"email_change_ownership_id,omitempty"`
}

func snapshotClaim(claim sitemanagement.Claim) claimSnapshot {
	return claimSnapshot{
		UserID: claim.UserID, Address: claim.Address, Method: string(claim.Method), TokenHash: claim.TokenHash,
		EmailChangeID: claim.EmailChangeID, EmailChangeOwnershipID: claim.EmailChangeOwnershipID,
	}
}

func (r *Repository) EnqueueClaim(ctx context.Context, claim sitemanagement.Claim) (sitemanagement.VerificationTask, error) {
	var result sitemanagement.VerificationTask
	err := r.transaction(ctx, func(tx pgx.Tx) error {
		if err := taskLock(ctx, tx); err != nil {
			return err
		}
		current, err := sitemanagement.WithTransaction(tx).Claim(ctx, claim.ID)
		if err != nil {
			return err
		}
		if current.UserID != claim.UserID || current.Status != "PENDING" || snapshotClaim(current) != snapshotClaim(claim) ||
			current.ExpiresAt == nil || !r.now().Before(*current.ExpiresAt) || (current.Method != sitemanagement.File && current.Method != sitemanagement.Meta) {
			return conflict()
		}
		digest, err := hex.DecodeString(current.TokenHash)
		if err != nil {
			return err
		}
		err = tx.QueryRow(ctx, `SELECT r.id::text,r.state FROM operations.targets t JOIN operations.runs r ON r.id=t.run_id
			WHERE t.claim_id=$1 AND t.challenge_digest=$2 AND r.state IN ('queued','running','pausing','paused','cancelling')`,
			claim.ID, digest).Scan(&result.ID, &result.State)
		if err == nil {
			return nil
		}
		if !noRows(err) {
			return err
		}
		var configID string
		if err := tx.QueryRow(ctx, `SELECT verification_config_id::text FROM operations.task_settings WHERE id=1`).Scan(&configID); err != nil {
			return err
		}
		c, err := loadConfiguration(ctx, tx, configID, "verification")
		if err != nil {
			return err
		}
		if !c.Signed {
			return unsignedDenied()
		}
		c.RequestConfigID, c.Concurrency, c.RunTimeoutSeconds = configID, 1, 86400
		c.TimeoutSeconds = min(c.TimeoutSeconds, 8)
		result.ID, err = r.createRun(ctx, tx, newRun{Kind: "verification", Configuration: c, At: r.now(), Source: "event", Actor: Actor{ID: current.UserID}})
		if err != nil {
			return err
		}
		address := current.Address
		if current.Method == sitemanagement.File {
			address = strings.TrimRight(address, "/") + "/.well-known/heyblog-site-verification.txt"
		}
		snapshot := snapshotClaim(current)
		_, err = tx.Exec(ctx, `INSERT INTO operations.targets(run_id,site_id,kind,address,generation,access_scope,claim_id,challenge_digest,claim_snapshot)
			SELECT $1,id,'verification',$2,monitoring_generation,access_scope,$3,$4,$5 FROM directory.sites WHERE id=$6`,
			result.ID, address, current.ID, digest, snapshot, current.SiteID)
		result.State = "queued"
		return err
	})
	result.StatusURL = "/api/web/account/site-claims/" + claim.ID + "/task"
	return result, err
}

func (r *Repository) ClaimTask(ctx context.Context, claimID, userID string) (*sitemanagement.VerificationTask, error) {
	var result sitemanagement.VerificationTask
	err := r.db.QueryRow(ctx, `SELECT r.id::text,r.state,COALESCE(NULLIF(t.reason,''),r.reason)
		FROM operations.targets t JOIN operations.runs r ON r.id=t.run_id JOIN directory.site_claims c ON c.id=t.claim_id
		WHERE c.id=$1 AND c.user_id=$2 ORDER BY r.created_at DESC,r.id DESC LIMIT 1`, claimID, userID).
		Scan(&result.ID, &result.State, &result.Reason)
	if noRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result.StatusURL = "/api/web/account/site-claims/" + claimID + "/task"
	return &result, nil
}

func (e *Execution) verifyClaim(ctx context.Context, tx pgx.Tx, l Lease, status int, body []byte) (bool, error) {
	if l.Target.ClaimID == nil {
		return false, errStaleTarget
	}
	repository := sitemanagement.WithTransaction(tx)
	claim, err := repository.Claim(ctx, *l.Target.ClaimID)
	if err != nil {
		return false, err
	}
	var expected claimSnapshot
	if err := tx.QueryRow(ctx, `SELECT claim_snapshot FROM operations.targets WHERE id=$1`, l.Target.ID).Scan(&expected); err != nil {
		return false, err
	}
	if claim.Status != "PENDING" || snapshotClaim(claim) != expected ||
		claim.ExpiresAt == nil || !e.repository.now().Before(*claim.ExpiresAt) {
		return false, errStaleTarget
	}
	if !sitemanagement.CheckHTTPProof(claim, status, body) {
		return false, nil
	}
	_, err = repository.Decide(ctx, sitemanagement.Decision{Claim: claim, Status: "VERIFIED", Now: e.repository.now()})
	var failure *sitemanagement.ServiceError
	if errors.As(err, &failure) || errors.Is(err, sitemanagement.ErrNotFound) {
		return false, errStaleTarget
	}
	return err == nil, err
}
