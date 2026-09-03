package mail

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type recordingSender struct {
	message Message
	err     error
}

func (sender *recordingSender) Send(_ context.Context, message Message) error {
	sender.message = message
	return sender.err
}

func TestVerificationMailerSendsPlainTextCodeFromVerificationIdentity(t *testing.T) {
	t.Parallel()

	sender := &recordingSender{}
	mailer := NewVerificationMailer(sender, "no-reply@verify.mail.heyblog.net", 10*time.Minute)
	err := mailer.SendVerificationCode(context.Background(), "reader@example.test", "123456")
	if err != nil {
		t.Fatalf("SendVerificationCode() error = %v", err)
	}

	message := sender.message
	if message.From != "no-reply@verify.mail.heyblog.net" || message.To != "reader@example.test" {
		t.Fatalf("addresses = (%q, %q), want verification sender and reader", message.From, message.To)
	}
	if message.Subject != verificationSubject || !strings.Contains(message.Text, "123456") {
		t.Fatalf("message = %#v, want verification subject and code", message)
	}
	if message.HTML != "" {
		t.Fatalf("HTML = %q, want plain text only for the initial verification flow", message.HTML)
	}
	if !strings.Contains(message.Text, "10 分钟") {
		t.Fatalf("Text = %q, want configured validity period", message.Text)
	}
}

func TestVerificationMailerRejectsEmptyCode(t *testing.T) {
	t.Parallel()

	sender := &recordingSender{}
	mailer := NewVerificationMailer(sender, "no-reply@verify.mail.heyblog.net", 10*time.Minute)
	if err := mailer.SendVerificationCode(context.Background(), "reader@example.test", "  "); err == nil {
		t.Fatal("SendVerificationCode() error = nil, want empty code error")
	}
	if sender.message != (Message{}) {
		t.Fatal("mail sender was called for an empty verification code")
	}
}

func TestVerificationMailerWrapsSenderFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("send failed")
	mailer := NewVerificationMailer(&recordingSender{err: wantErr}, "no-reply@verify.mail.heyblog.net", 10*time.Minute)
	err := mailer.SendVerificationCode(context.Background(), "reader@example.test", "123456")
	if !errors.Is(err, wantErr) {
		t.Fatalf("SendVerificationCode() error = %v, want sender failure", err)
	}
	if strings.Contains(err.Error(), "123456") {
		t.Fatalf("SendVerificationCode() error leaked verification code: %v", err)
	}
}
