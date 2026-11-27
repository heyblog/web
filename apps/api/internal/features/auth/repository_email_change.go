package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type emailChangeTransaction struct {
	tx      pgx.Tx
	queries *dbgen.Queries
	user    dbUser
	userID  pgtype.UUID
	pending dbgen.IdentityEmailChangeRequest
}

func (repo *repository) beginEmailChange(ctx context.Context, actor User) (emailChangeTransaction, error) {
	id, err := parseUUID(actor.ID)
	if err != nil {
		return emailChangeTransaction{}, err
	}
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return emailChangeTransaction{}, err
	}
	change := emailChangeTransaction{tx: tx, queries: repo.queries.WithTx(tx), userID: id}
	row, err := change.queries.GetUserForUpdate(ctx, id)
	if err != nil {
		change.rollback(ctx)
		return emailChangeTransaction{}, err
	}
	change.user = mapUser(row)
	if !isActive(change.user) || change.user.DeletionRequestedAt != nil || change.user.AuthVersion != actor.AuthVersion || change.user.ProfileConfirmationStatus == "PENDING" {
		change.rollback(ctx)
		return emailChangeTransaction{}, newAuthError("unauthenticated", http.StatusUnauthorized, "account authentication changed")
	}
	if change.user.VerifiedAt == nil {
		change.rollback(ctx)
		return emailChangeTransaction{}, newAuthError("email_not_verified", http.StatusForbidden, "the current email address must be verified")
	}
	change.pending, err = change.queries.LockEmailChangeRequest(ctx, id)
	if err != nil && !isNotFound(err) {
		change.rollback(ctx)
		return emailChangeTransaction{}, err
	}
	return change, nil
}

func (change emailChangeTransaction) rollback(ctx context.Context) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = change.tx.Rollback(cleanup)
}

func (change emailChangeTransaction) active(now time.Time) bool {
	return change.pending.ID.Valid && change.pending.AuthVersion == change.user.AuthVersion &&
		change.pending.OldEmail == *change.user.Email && now.Before(change.pending.ExpiresAt.Time)
}

func (change emailChangeTransaction) require(requestID string) error {
	expected, err := parseUUID(requestID)
	if err != nil || !expected.Valid || expected != change.pending.ID {
		return newAuthError("email_change_missing", http.StatusNotFound, "the email change request was not found")
	}
	if !change.active(time.Now()) {
		return newAuthError("email_change_expired", http.StatusConflict, "the email change request expired or the account changed")
	}
	return nil
}

func (change *emailChangeTransaction) close(ctx context.Context) error {
	if err := change.queries.CloseEmailChangeSiteClaims(ctx, change.pending.ID); err != nil {
		return err
	}
	if err := change.queries.DeleteEmailChangeRequest(ctx, change.pending.ID); err != nil {
		return err
	}
	change.pending = dbgen.IdentityEmailChangeRequest{}
	return nil
}

func (change emailChangeTransaction) status(ctx context.Context) (EmailChangeStatus, error) {
	list, err := change.queries.ListEmailChangeSites(ctx, dbgen.ListEmailChangeSitesParams{UserID: change.userID, RequestID: change.pending.ID})
	if err != nil {
		return EmailChangeStatus{}, err
	}
	status := EmailChangeStatus{Sites: make([]EmailChangeSite, 0, len(list))}
	if change.pending.ID.Valid {
		status.Request = &EmailChangeRequest{
			ID: change.pending.ID.String(), NewEmail: change.pending.NewEmail, ExpiresAt: change.pending.ExpiresAt.Time,
			OldVerified: change.pending.OldVerified, NewVerified: change.pending.NewVerified,
		}
	}
	for _, site := range list {
		status.Sites = append(status.Sites, EmailChangeSite{ShortID: site.ShortID, Name: site.Name, Address: site.Address, Status: site.Status, ClaimID: site.ClaimID})
	}
	return status, nil
}

func (change emailChangeTransaction) commitStatus(ctx context.Context) (EmailChangeStatus, error) {
	status, err := change.status(ctx)
	if err != nil {
		return EmailChangeStatus{}, err
	}
	if err := change.tx.Commit(ctx); err != nil {
		return EmailChangeStatus{}, err
	}
	return status, nil
}

func (repo *repository) emailChangeStatus(ctx context.Context, actor User) (EmailChangeStatus, error) {
	change, err := repo.beginEmailChange(ctx, actor)
	if err != nil {
		return EmailChangeStatus{}, err
	}
	defer change.rollback(ctx)
	if change.pending.ID.Valid && !change.active(time.Now()) {
		if err := change.close(ctx); err != nil {
			return EmailChangeStatus{}, err
		}
	}
	return change.commitStatus(ctx)
}

func (repo *repository) startEmailChange(ctx context.Context, actor User, email string) (EmailChangeStatus, error) {
	change, err := repo.beginEmailChange(ctx, actor)
	if err != nil {
		return EmailChangeStatus{}, err
	}
	defer change.rollback(ctx)
	if email == *change.user.Email {
		return EmailChangeStatus{}, newAuthError("email_unchanged", http.StatusUnprocessableEntity, "the new email matches the current email")
	}
	if change.pending.ID.Valid {
		if change.active(time.Now()) {
			if email == change.pending.NewEmail {
				return change.commitStatus(ctx)
			}
			return EmailChangeStatus{}, newAuthError("email_change_active", http.StatusConflict, "cancel the current email change request before starting another")
		}
		if err := change.close(ctx); err != nil {
			return EmailChangeStatus{}, err
		}
	}
	if _, err := change.queries.GetUserByEmail(ctx, email); err == nil {
		return EmailChangeStatus{}, newAuthError("email_taken", http.StatusConflict, "the email is already in use")
	} else if !isNotFound(err) {
		return EmailChangeStatus{}, err
	}
	change.pending, err = change.queries.CreateEmailChangeRequest(ctx, dbgen.CreateEmailChangeRequestParams{
		UserID: change.userID, AuthVersion: change.user.AuthVersion, OldEmail: *change.user.Email, NewEmail: email,
	})
	if err != nil {
		return EmailChangeStatus{}, err
	}
	return change.commitStatus(ctx)
}

func (repo *repository) cancelEmailChange(ctx context.Context, actor User, requestID string) (EmailChangeStatus, error) {
	change, err := repo.beginEmailChange(ctx, actor)
	if err != nil {
		return EmailChangeStatus{}, err
	}
	defer change.rollback(ctx)
	expected, err := parseUUID(requestID)
	if err != nil || !expected.Valid || expected != change.pending.ID {
		return EmailChangeStatus{}, newAuthError("email_change_missing", http.StatusNotFound, "the email change request was not found")
	}
	if err := change.close(ctx); err != nil {
		return EmailChangeStatus{}, err
	}
	return change.commitStatus(ctx)
}

func (repo *repository) completeEmailChange(ctx context.Context, actor User, requestID string) (dbUser, string, error) {
	change, err := repo.beginEmailChange(ctx, actor)
	if err != nil {
		return dbUser{}, "", err
	}
	defer change.rollback(ctx)
	if err := change.require(requestID); err != nil {
		return dbUser{}, "", err
	}
	if !change.pending.OldVerified || !change.pending.NewVerified {
		return dbUser{}, "", newAuthError("email_change_verification_required", http.StatusConflict, "email change verification is required")
	}
	complete, err := change.queries.CompleteEmailChangeSiteProofs(ctx, dbgen.CompleteEmailChangeSiteProofsParams{
		UserID: change.userID, RequestID: change.pending.ID,
	})
	if err != nil {
		return dbUser{}, "", err
	}
	if !complete {
		if err := change.require(requestID); err != nil {
			return dbUser{}, "", err
		}
		return dbUser{}, "", newAuthError("email_change_sites_required", http.StatusConflict, "all current sites must be reverified")
	}
	updated, err := change.queries.ChangeUserEmail(ctx, change.pending.ID)
	if err != nil {
		return dbUser{}, "", identityError(err)
	}
	if updated == 0 {
		return dbUser{}, "", newAuthError("email_change_expired", http.StatusConflict, "the email change request expired or the account changed")
	}
	// The users trigger clears verification on an email change, then both writes commit together.
	if err := change.queries.SetUserEmailVerified(ctx, change.userID); err != nil {
		return dbUser{}, "", err
	}
	if err := change.queries.DeleteEmailVerificationCodes(ctx, change.userID); err != nil {
		return dbUser{}, "", err
	}
	if err := change.queries.DeletePasswordResetTokens(ctx, change.userID); err != nil {
		return dbUser{}, "", err
	}
	if err := change.close(ctx); err != nil {
		return dbUser{}, "", err
	}
	record, err := change.queries.GetUserByID(ctx, change.userID)
	if err != nil {
		return dbUser{}, "", err
	}
	return mapUser(record), *change.user.Email, change.tx.Commit(ctx)
}
