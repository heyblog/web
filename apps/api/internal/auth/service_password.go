package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"heyblog-api/internal/mail"
)

func (service *Service) ForgotPassword(ctx context.Context, email string) error {
	email = normalizeEmail(email)
	user, err := service.repo.userByEmail(ctx, email)
	if isNotFound(err) {
		return nil
	}
	if err != nil {
		return repositoryError("find reset user", err)
	}
	if user.PasswordHash == nil || user.VerifiedAt == nil || !isActive(user) {
		return nil
	}
	token, err := randomToken(32)
	if err != nil {
		return repositoryError("generate reset token", err)
	}
	if err := service.repo.createResetToken(ctx, user.ID, email, digestToken(service.config.RefreshSecret, token), time.Now().Add(service.config.PasswordResetTTL)); err != nil {
		return repositoryError("store reset token", err)
	}
	if service.sender == nil {
		return newAuthError("mail_unavailable", 503, "email delivery is unavailable")
	}
	url := strings.TrimRight(service.config.WebBaseURL, "/") + "/reset-password?token=" + token
	text := "请使用以下链接重置密码：\n\n" + url + "\n\n链接将在 " + mail.FormatValidity(service.config.PasswordResetTTL) + "后过期。"
	if err := service.sender.Send(ctx, mail.Message{From: service.config.MailFrom, To: email, Subject: "HeyBlog 密码重置", Text: text}); err != nil {
		return repositoryError("send reset email", err)
	}
	return nil
}

func (service *Service) ResetPassword(ctx context.Context, token, password string) error {
	if !validPassword(password) {
		return newAuthError("invalid_password", 422, "password is invalid")
	}
	hash, err := hashPassword(password)
	if err != nil {
		return repositoryError("hash reset password", err)
	}
	if err := service.repo.resetPasswordWithToken(ctx, digestToken(service.config.RefreshSecret, token), hash); err != nil {
		return repositoryError("update reset password", err)
	}
	return nil
}

func (service *Service) SetPassword(ctx context.Context, request *http.Request, current, next string) (User, []string, error) {
	user, err := service.Current(ctx, request)
	if err != nil {
		return User{}, nil, err
	}
	record, err := service.repo.userByID(ctx, user.ID)
	if err != nil {
		return User{}, nil, repositoryError("read password user", err)
	}
	if record.PasswordHash != nil && (current == "" || !verifyPassword(current, *record.PasswordHash)) {
		return User{}, nil, newAuthError("invalid_current_password", 403, "current password is invalid")
	}
	if !validPassword(next) {
		return User{}, nil, newAuthError("invalid_password", 422, "password is invalid")
	}
	hash, err := hashPassword(next)
	if err != nil {
		return User{}, nil, repositoryError("hash new password", err)
	}
	if err := service.repo.updatePassword(ctx, user.ID, hash); err != nil {
		return User{}, nil, repositoryError("update password", err)
	}
	updated, err := service.repo.userByID(ctx, user.ID)
	if err != nil {
		return User{}, nil, repositoryError("read updated user", err)
	}
	access, refresh, err := service.issueSession(ctx, updated)
	if err != nil {
		return User{}, nil, err
	}
	authUser, err := service.toUser(ctx, updated)
	return authUser, []string{access, refresh}, err
}
