package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"heyblog-api/internal/infrastructure/mail"
	"heyblog-api/internal/platform/apperror"
	"heyblog-api/internal/platform/httpapi"
)

const emailChangeMaxCodeAttempts = 5

type EmailChangeStatus struct {
	Request *EmailChangeRequest `json:"request"`
	Sites   []EmailChangeSite   `json:"sites"`
}

type EmailChangeRequest struct {
	ID          string    `json:"id"`
	NewEmail    string    `json:"new_email"`
	ExpiresAt   time.Time `json:"expires_at"`
	OldVerified bool      `json:"old_verified"`
	NewVerified bool      `json:"new_verified"`
}

type EmailChangeSite struct {
	ShortID string `json:"short_id"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Status  string `json:"status" enum:"required,pending,verified,rejected"`
	ClaimID string `json:"claim_id"`
}

type EmailChangeSide string

const (
	EmailChangeOld EmailChangeSide = "old"
	EmailChangeNew EmailChangeSide = "new"
)

func (side EmailChangeSide) valid() bool {
	return side == EmailChangeOld || side == EmailChangeNew
}

func invalidEmailChangeCode() *AuthError {
	return newAuthError("invalid_verification_code", http.StatusBadRequest, "the email change code is invalid")
}

func (service *Service) EmailChange(ctx context.Context, request *http.Request) (EmailChangeStatus, error) {
	actor, err := service.Current(ctx, request)
	if err != nil {
		return EmailChangeStatus{}, err
	}
	return service.repo.emailChangeStatus(ctx, actor)
}

func (service *Service) StartEmailChange(ctx context.Context, request *http.Request, email string) (EmailChangeStatus, error) {
	actor, err := service.Current(ctx, request)
	if err != nil {
		return EmailChangeStatus{}, err
	}
	address := normalizeEmail(email)
	if !validEmail(address) {
		return EmailChangeStatus{}, newAuthError("invalid_email", http.StatusUnprocessableEntity, "the new email address is invalid")
	}
	return service.repo.startEmailChange(ctx, actor, address)
}

func (service *Service) CancelEmailChange(ctx context.Context, request *http.Request, requestID string) (EmailChangeStatus, error) {
	actor, err := service.Current(ctx, request)
	if err != nil {
		return EmailChangeStatus{}, err
	}
	return service.repo.cancelEmailChange(ctx, actor, requestID)
}

func (service *Service) SendEmailChangeCode(ctx context.Context, request *http.Request, requestID string, side EmailChangeSide, allowDelivery func(string, string) error) (EmailChangeStatus, error) {
	actor, err := service.Current(ctx, request)
	if err != nil {
		return EmailChangeStatus{}, err
	}
	if !side.valid() {
		return EmailChangeStatus{}, newAuthError("invalid_email_change_side", http.StatusUnprocessableEntity, "the email address to verify is invalid")
	}
	if service.sender == nil {
		return EmailChangeStatus{}, newAuthError("mail_unavailable", http.StatusServiceUnavailable, "email delivery is unavailable")
	}
	code, err := randomDigits(6)
	if err != nil {
		return EmailChangeStatus{}, err
	}
	hash := service.emailChangeCodeHash(requestID, side, code)
	recipient, status, err := service.repo.issueEmailChangeCode(ctx, actor, requestID, side, hash, service.config.VerificationTTL, allowDelivery)
	if err != nil || recipient == "" {
		return status, err
	}
	ttl := mail.FormatValidity(service.config.VerificationTTL)
	title := "更换账号邮箱验证码"
	details := "正在验证新邮箱。"
	if side == EmailChangeOld {
		details = "正在验证当前邮箱。"
	}
	message := details + "\n\n验证码为 " + code + "，将在 " + ttl + "后过期。请勿向他人透露验证码。如果这次操作并非由你发起，请忽略此邮件。"
	if err := service.sendEmailChangeMail(ctx, recipient, title, message); err != nil {
		return EmailChangeStatus{}, err
	}
	return status, nil
}

func (service *Service) VerifyEmailChangeCode(ctx context.Context, request *http.Request, requestID string, side EmailChangeSide, code string) (EmailChangeStatus, error) {
	actor, err := service.Current(ctx, request)
	if err != nil {
		return EmailChangeStatus{}, err
	}
	if !side.valid() || len(code) != 6 || strings.ContainsFunc(code, func(ch rune) bool { return ch < '0' || ch > '9' }) {
		return EmailChangeStatus{}, invalidEmailChangeCode()
	}
	return service.repo.verifyEmailChangeCode(ctx, actor, requestID, side, service.emailChangeCodeHash(requestID, side, code))
}

func (service *Service) VerifyEmailChangePassword(ctx context.Context, request *http.Request, requestID, password string, allowAttempt func(string) error) (EmailChangeStatus, error) {
	actor, err := service.Current(ctx, request)
	if err != nil {
		return EmailChangeStatus{}, err
	}
	if err := allowAttempt(actor.ID); err != nil {
		return EmailChangeStatus{}, err
	}
	if !validPassword(password) {
		return EmailChangeStatus{}, invalidEmailChangePassword()
	}
	return service.repo.verifyEmailChangePassword(ctx, actor, requestID, password)
}

func (service *Service) CompleteEmailChange(ctx context.Context, request *http.Request, requestID string) (User, []string, error) {
	actor, err := service.Current(ctx, request)
	if err != nil {
		return User{}, nil, err
	}
	record, oldEmail, err := service.repo.completeEmailChange(ctx, actor, requestID)
	if err != nil {
		return User{}, nil, err
	}
	user, cookies, sessionErr := service.profileResult(ctx, record, true)
	noticeContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if err := service.sendEmailChangeMail(noticeContext, oldEmail, "账号邮箱已更换", "你的 HeyBlog 账号已更换登录邮箱。其他设备需要重新登录。如果这次操作并非由你发起，请联系站点管理员。"); err != nil {
		httpapi.LogError(ctx, mapError(err))
	}
	return user, cookies, sessionErr
}

func (service *Service) emailChangeCodeHash(requestID string, side EmailChangeSide, code string) string {
	return digestToken(service.config.AccessSecret, "email-change:"+strings.ToLower(requestID)+":"+string(side)+":"+code)
}

func (service *Service) sendEmailChangeMail(ctx context.Context, recipient, title, message string) error {
	if service.sender == nil {
		return newAuthError("mail_unavailable", http.StatusServiceUnavailable, "email delivery is unavailable")
	}
	rendered, err := mail.RenderDefaultEmail(mail.EmailTemplate{Title: title, Message: message, Preheader: title})
	if err != nil {
		return repositoryError("render email change message", err)
	}
	if err := service.sender.Send(ctx, mail.Message{From: service.config.MailFrom, To: recipient, Subject: "HeyBlog " + title, Text: message, HTML: rendered.HTML, InlineImage: rendered.InlineImage}); err != nil {
		return apperror.Wrap(err, apperror.KindUnavailable, "mail_unavailable", "email delivery is unavailable", "send email change message")
	}
	return nil
}
