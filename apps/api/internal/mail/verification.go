package mail

import (
	"context"
	"fmt"
	"strings"
)

const verificationSubject = "HeyBlog 邮箱验证码"

type VerificationMailer struct {
	sender Sender
	from   string
}

func NewVerificationMailer(sender Sender, from string) *VerificationMailer {
	return &VerificationMailer{sender: sender, from: from}
}

func (mailer *VerificationMailer) SendVerificationCode(ctx context.Context, recipient, code string) error {
	if strings.TrimSpace(code) == "" {
		return fmt.Errorf("verification code is required")
	}
	text := fmt.Sprintf(
		"您的 HeyBlog 邮箱验证码是：\n\n%s\n\n如果该操作并非您本人发起，请忽略此邮件。",
		code,
	)
	if err := mailer.sender.Send(ctx, Message{
		From:    mailer.from,
		To:      recipient,
		Subject: verificationSubject,
		Text:    text,
	}); err != nil {
		return fmt.Errorf("send verification email: %w", err)
	}
	return nil
}
