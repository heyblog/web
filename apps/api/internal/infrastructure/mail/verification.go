package mail

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const verificationSubject = "HeyBlog 邮箱验证码"

type VerificationMailer struct {
	sender   Sender
	from     string
	validity time.Duration
}

func NewVerificationMailer(sender Sender, from string, validity time.Duration) *VerificationMailer {
	return &VerificationMailer{sender: sender, from: from, validity: validity}
}

func (mailer *VerificationMailer) SendVerificationCode(ctx context.Context, recipient, code string) error {
	if strings.TrimSpace(code) == "" {
		return fmt.Errorf("verification code is required")
	}
	text := fmt.Sprintf(
		"您的 HeyBlog 邮箱验证码是：\n\n%s\n\n验证码将在 %s后过期。如果该操作并非您本人发起，请忽略此邮件。",
		code,
		FormatValidity(mailer.validity),
	)
	html, err := RenderDefaultEmail(EmailTemplate{
		Title:     "邮箱验证码",
		Message:   "您的 HeyBlog 邮箱验证码是：\n\n" + code + "\n\n验证码将在 " + FormatValidity(mailer.validity) + "后过期。如果该操作并非您本人发起，请忽略此邮件。",
		Preheader: "您的 HeyBlog 邮箱验证码",
		Details:   []EmailDetail{{Label: "验证码", Value: code}, {Label: "有效期", Value: FormatValidity(mailer.validity)}},
	})
	if err != nil {
		return fmt.Errorf("render verification email: %w", err)
	}
	if err := mailer.sender.Send(ctx, Message{
		From:    mailer.from,
		To:      recipient,
		Subject: verificationSubject,
		Text:    text,
		HTML:    html,
	}); err != nil {
		return fmt.Errorf("send verification email: %w", err)
	}
	return nil
}
