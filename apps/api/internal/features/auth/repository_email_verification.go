package auth

import (
	"context"
	"crypto/subtle"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type emailChangeCode struct {
	recipient string
	hash      *string
	expiresAt time.Time
	attempts  int32
	verified  bool
}

func getEmailChangeCode(change dbgen.IdentityEmailChangeRequest, side EmailChangeSide) emailChangeCode {
	if side == EmailChangeOld {
		return emailChangeCode{change.OldEmail, change.OldCodeHash, change.OldCodeExpiresAt.Time, change.OldAttempts, change.OldVerified}
	}
	return emailChangeCode{change.NewEmail, change.NewCodeHash, change.NewCodeExpiresAt.Time, change.NewAttempts, change.NewVerified}
}

func (repo *repository) issueEmailChangeCode(ctx context.Context, actor User, requestID string, side EmailChangeSide, hash string, ttl time.Duration, allowDelivery func(string, string) error) (string, EmailChangeStatus, error) {
	change, err := repo.beginEmailChange(ctx, actor)
	if err != nil {
		return "", EmailChangeStatus{}, err
	}
	defer change.rollback(ctx)
	if err := change.require(requestID); err != nil {
		return "", EmailChangeStatus{}, err
	}
	current := getEmailChangeCode(change.pending, side)
	if current.verified {
		status, err := change.commitStatus(ctx)
		return "", status, err
	}
	if err := allowDelivery(actor.ID, current.recipient); err != nil {
		return "", EmailChangeStatus{}, err
	}
	if err := change.queries.IssueEmailChangeCode(ctx, dbgen.IssueEmailChangeCodeParams{
		ID: change.pending.ID, IsOld: side == EmailChangeOld, CodeHash: hash, CodeExpiry: timestamp(time.Now().Add(ttl)),
	}); err != nil {
		return "", EmailChangeStatus{}, err
	}
	status, err := change.commitStatus(ctx)
	return current.recipient, status, err
}

func (repo *repository) verifyEmailChangeCode(ctx context.Context, actor User, requestID string, side EmailChangeSide, hash string) (EmailChangeStatus, error) {
	change, err := repo.beginEmailChange(ctx, actor)
	if err != nil {
		return EmailChangeStatus{}, err
	}
	defer change.rollback(ctx)
	if err := change.require(requestID); err != nil {
		return EmailChangeStatus{}, err
	}
	current := getEmailChangeCode(change.pending, side)
	if current.hash == nil || !time.Now().Before(current.expiresAt) || current.attempts >= emailChangeMaxCodeAttempts {
		return EmailChangeStatus{}, invalidEmailChangeCode()
	}
	if subtle.ConstantTimeCompare([]byte(*current.hash), []byte(hash)) != 1 {
		if err := change.queries.IncrementEmailChangeCodeAttempts(ctx, dbgen.IncrementEmailChangeCodeAttemptsParams{
			ID: change.pending.ID, IsOld: side == EmailChangeOld,
		}); err != nil {
			return EmailChangeStatus{}, err
		}
		if err := change.tx.Commit(ctx); err != nil {
			return EmailChangeStatus{}, err
		}
		return EmailChangeStatus{}, invalidEmailChangeCode()
	}
	return change.markVerified(ctx, side)
}

func (change *emailChangeTransaction) markVerified(ctx context.Context, side EmailChangeSide) (EmailChangeStatus, error) {
	updated, err := change.queries.MarkEmailChangeVerified(ctx, dbgen.MarkEmailChangeVerifiedParams{
		ID: change.pending.ID, IsOld: side == EmailChangeOld,
	})
	if err != nil {
		return EmailChangeStatus{}, err
	}
	change.pending = updated
	return change.commitStatus(ctx)
}

func invalidEmailChangePassword() *AuthError {
	return newAuthError("invalid_current_password", http.StatusForbidden, "the current password is invalid")
}

func (repo *repository) verifyEmailChangePassword(ctx context.Context, actor User, requestID, password string) (EmailChangeStatus, error) {
	user, err := repo.userByID(ctx, actor.ID)
	if err != nil {
		return EmailChangeStatus{}, err
	}
	if user.PasswordHash == nil || !verifyPassword(password, *user.PasswordHash) {
		return EmailChangeStatus{}, invalidEmailChangePassword()
	}
	change, err := repo.beginEmailChange(ctx, actor)
	if err != nil {
		return EmailChangeStatus{}, err
	}
	defer change.rollback(ctx)
	if err := change.require(requestID); err != nil {
		return EmailChangeStatus{}, err
	}
	unproven, err := change.queries.HasUnprovenEmailChangePassword(ctx, change.userID)
	if err != nil {
		return EmailChangeStatus{}, err
	}
	if unproven {
		return EmailChangeStatus{}, newAuthError("email_change_password_unverified", http.StatusConflict, "the password requires verified email recovery before it can authorize an email change")
	}
	return change.markVerified(ctx, EmailChangeOld)
}

func (repo *repository) validateEmailChangeOAuth(ctx context.Context, actor User, requestID string, provider OAuthProvider) error {
	change, err := repo.beginEmailChange(ctx, actor)
	if err != nil {
		return err
	}
	defer change.rollback(ctx)
	if err := change.require(requestID); err != nil {
		return err
	}
	_, err = boundEmailChangeSubject(ctx, change.queries, change.userID, provider)
	return err
}

func (repo *repository) verifyEmailChangeOAuth(ctx context.Context, actor User, requestID string, provider OAuthProvider, subject string) error {
	change, err := repo.beginEmailChange(ctx, actor)
	if err != nil {
		return err
	}
	defer change.rollback(ctx)
	if err := change.require(requestID); err != nil {
		return err
	}
	expected, err := boundEmailChangeSubject(ctx, change.queries, change.userID, provider)
	if err != nil {
		return err
	}
	if expected != subject {
		return newAuthError("oauth_identity_not_bound", http.StatusForbidden, "the OAuth identity is not linked to this account")
	}
	_, err = change.markVerified(ctx, EmailChangeOld)
	return err
}

func boundEmailChangeSubject(ctx context.Context, queries *dbgen.Queries, userID pgtype.UUID, provider OAuthProvider) (string, error) {
	identity, err := queries.GetUserProviderIdentity(ctx, dbgen.GetUserProviderIdentityParams{UserID: userID, Provider: string(provider)})
	if isNotFound(err) {
		return "", newAuthError("oauth_identity_not_bound", http.StatusForbidden, "the OAuth identity is not linked to this account")
	}
	return identity.ProviderUserID, err
}
