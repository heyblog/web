package mail

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"
)

type fakeSESClient struct {
	ctx   context.Context
	input *sesv2.SendEmailInput
	err   error
}

func TestOpenSESLoadsCredentialsFromEnvironment(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-key")
	t.Setenv("AWS_SESSION_TOKEN", "test-session-token")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "missing-credentials"))
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "missing-config"))

	sender, err := OpenSES(context.Background(), "ap-southeast-1")
	if err != nil {
		t.Fatalf("OpenSES() error = %v, want environment credentials to initialize SES", err)
	}
	if sender == nil {
		t.Fatal("OpenSES() sender = nil, want initialized sender")
	}
}

func TestOpenSESRejectsMissingCredentials(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_SESSION_TOKEN", "")
	t.Setenv("AWS_PROFILE", "missing-profile")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "missing-credentials"))
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "missing-config"))

	_, err := OpenSES(context.Background(), "ap-southeast-1")
	if err == nil {
		t.Fatal("OpenSES() error = nil, want missing credentials error")
	}
	var credentialError *sesCredentialError
	if !errors.As(err, &credentialError) {
		t.Fatalf("OpenSES() error = %T, want SES credential error", err)
	}
	if credentialError.Error() != "load AWS credentials for SES" {
		t.Fatalf("credential error = %q, want stable non-secret message", credentialError.Error())
	}
}

func (client *fakeSESClient) SendEmail(
	ctx context.Context,
	input *sesv2.SendEmailInput,
	_ ...func(*sesv2.Options),
) (*sesv2.SendEmailOutput, error) {
	client.ctx = ctx
	client.input = input
	return &sesv2.SendEmailOutput{}, client.err
}

func TestSESSenderBuildsUTF8SimpleMessage(t *testing.T) {
	t.Parallel()

	client := &fakeSESClient{}
	sender := newSESSender(client)
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "request-context")
	err := sender.Send(ctx, Message{
		From:    "no-reply@verify.mail.heyblog.net",
		To:      "reader@example.test",
		Subject: "HeyBlog 邮箱验证码",
		Text:    "纯文本验证码：123456",
		HTML:    "<p>HTML 验证码：<strong>123456</strong></p>",
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if client.ctx.Value(contextKey{}) != "request-context" {
		t.Fatal("SendEmail() did not receive the caller context")
	}

	input := client.input
	if input == nil || input.FromEmailAddress == nil || *input.FromEmailAddress != "no-reply@verify.mail.heyblog.net" {
		t.Fatalf("FromEmailAddress = %v, want configured verification sender", input)
	}
	if input.Destination == nil || len(input.Destination.ToAddresses) != 1 || input.Destination.ToAddresses[0] != "reader@example.test" {
		t.Fatalf("Destination = %#v, want one reader recipient", input.Destination)
	}
	if input.Content == nil || input.Content.Simple == nil {
		t.Fatal("Content.Simple = nil, want a simple SES message")
	}
	message := input.Content.Simple
	if message.Subject == nil || message.Subject.Data == nil || *message.Subject.Data != "HeyBlog 邮箱验证码" ||
		message.Subject.Charset == nil || *message.Subject.Charset != utf8Charset {
		t.Fatalf("Subject = %#v, want UTF-8 Chinese subject", message.Subject)
	}
	if message.Body == nil || message.Body.Text == nil || message.Body.Html == nil {
		t.Fatalf("Body = %#v, want text and HTML alternatives", message.Body)
	}
	if message.Body.Text.Charset == nil || *message.Body.Text.Charset != utf8Charset ||
		message.Body.Html.Charset == nil || *message.Body.Html.Charset != utf8Charset {
		t.Fatalf("Body charsets = (%#v, %#v), want UTF-8", message.Body.Text, message.Body.Html)
	}
	if message.Body.Text.Data == nil || *message.Body.Text.Data != "纯文本验证码：123456" ||
		message.Body.Html.Data == nil || *message.Body.Html.Data != "<p>HTML 验证码：<strong>123456</strong></p>" {
		t.Fatalf("Body data = (%#v, %#v), want original text and HTML", message.Body.Text, message.Body.Html)
	}
}

func TestSESSenderSetsOnlyProvidedBodyAlternatives(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		text     string
		html     string
		wantText bool
		wantHTML bool
	}{
		"text only": {text: "plain text", wantText: true},
		"HTML only": {html: "<p>HTML</p>", wantHTML: true},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			client := &fakeSESClient{}
			err := newSESSender(client).Send(context.Background(), Message{
				From:    "sender@example.test",
				To:      "reader@example.test",
				Subject: "subject",
				Text:    test.text,
				HTML:    test.html,
			})
			if err != nil {
				t.Fatalf("Send() error = %v", err)
			}
			body := client.input.Content.Simple.Body
			if (body.Text != nil) != test.wantText || (body.Html != nil) != test.wantHTML {
				t.Fatalf("body alternatives = (text:%t, HTML:%t), want (text:%t, HTML:%t)",
					body.Text != nil,
					body.Html != nil,
					test.wantText,
					test.wantHTML,
				)
			}
		})
	}
}

func TestSESSenderRejectsInvalidMessagesWithoutCallingAWS(t *testing.T) {
	t.Parallel()

	tests := map[string]Message{
		"invalid from": {
			From: "not-an-address", To: "reader@example.test", Subject: "subject", Text: "body",
		},
		"invalid to": {
			From: "sender@example.test", To: "not-an-address", Subject: "subject", Text: "body",
		},
		"SMTPUTF8 to": {
			From: "sender@example.test", To: "用户@example.test", Subject: "subject", Text: "body",
		},
		"missing subject": {
			From: "sender@example.test", To: "reader@example.test", Text: "body",
		},
		"multiline subject": {
			From: "sender@example.test", To: "reader@example.test", Subject: "subject\ninjected", Text: "body",
		},
		"missing body": {
			From: "sender@example.test", To: "reader@example.test", Subject: "subject",
		},
	}
	for name, message := range tests {
		t.Run(name, func(t *testing.T) {
			client := &fakeSESClient{}
			err := newSESSender(client).Send(context.Background(), message)
			if err == nil {
				t.Fatal("Send() error = nil, want message validation error")
			}
			if client.input != nil {
				t.Fatal("SendEmail() was called for an invalid message")
			}
		})
	}
}

func TestSESSenderWrapsAWSFailureWithoutMessageContents(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("MessageRejected: private-recipient@example.test with code 123456")
	sender := newSESSender(&fakeSESClient{err: wantErr})
	err := sender.Send(context.Background(), Message{
		From:    "sender@example.test",
		To:      "private-recipient@example.test",
		Subject: "subject",
		Text:    "private verification code 123456",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Send() error = %v, want AWS failure", err)
	}
	if strings.Contains(err.Error(), "private-recipient") || strings.Contains(err.Error(), "123456") {
		t.Fatalf("Send() error leaked message contents: %v", err)
	}
	if !errors.Is(err, ErrDeliveryUnavailable) {
		t.Fatalf("Send() error = %v, want delivery-unavailable classification", err)
	}
	component, ok := err.(interface{ Component() string })
	if !ok || component.Component() != "ses" {
		t.Fatalf("Send() component = (%v, %t), want ses", component, ok)
	}
}
