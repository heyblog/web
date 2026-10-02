package mail

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

type smtpSender struct {
	address string
	timeout time.Duration
}

type smtpConnectionError struct {
	cause error
}

type smtpSendError struct {
	cause error
}

func OpenSMTP(ctx context.Context, address string, timeout time.Duration) (Sender, error) {
	if strings.TrimSpace(address) == "" {
		return nil, fmt.Errorf("SMTP address is required")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("SMTP timeout must be positive")
	}
	sender := newSMTPSender(address, timeout)
	if err := sender.withClient(ctx, func(client *smtp.Client) error {
		return client.Noop()
	}); err != nil {
		return nil, &smtpConnectionError{cause: err}
	}
	return sender, nil
}

func newSMTPSender(address string, timeout time.Duration) *smtpSender {
	return &smtpSender{address: address, timeout: timeout}
}

func (sender *smtpSender) Send(ctx context.Context, message Message) error {
	payload, err := renderSMTPMessage(message)
	if err != nil {
		return err
	}
	if err := sender.withClient(ctx, func(client *smtp.Client) error {
		if err := client.Mail(message.From); err != nil {
			return fmt.Errorf("set SMTP sender: %w", err)
		}
		if err := client.Rcpt(message.To); err != nil {
			return fmt.Errorf("set SMTP recipient: %w", err)
		}
		writer, err := client.Data()
		if err != nil {
			return fmt.Errorf("open SMTP message body: %w", err)
		}
		_, writeErr := writer.Write(payload)
		closeErr := writer.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			return fmt.Errorf("write SMTP message body: %w", err)
		}
		return nil
	}); err != nil {
		return &smtpSendError{cause: err}
	}
	return nil
}

func (sender *smtpSender) withClient(ctx context.Context, operation func(*smtp.Client) error) error {
	operationContext, cancel := context.WithTimeout(ctx, sender.timeout)
	defer cancel()

	connection, err := new(net.Dialer).DialContext(operationContext, "tcp", sender.address)
	if err != nil {
		return fmt.Errorf("connect SMTP service: %w", err)
	}
	deadline, ok := operationContext.Deadline()
	if ok {
		if err := connection.SetDeadline(deadline); err != nil {
			return errors.Join(fmt.Errorf("set SMTP deadline: %w", err), connection.Close())
		}
	}
	stopCancellation := context.AfterFunc(operationContext, func() {
		_ = connection.SetDeadline(time.Now())
	})
	defer stopCancellation()

	host, _, err := net.SplitHostPort(sender.address)
	if err != nil {
		return errors.Join(fmt.Errorf("parse SMTP address: %w", err), connection.Close())
	}
	client, err := smtp.NewClient(connection, host)
	if err != nil {
		return errors.Join(fmt.Errorf("initialize SMTP client: %w", err), connection.Close())
	}
	if err := operation(client); err != nil {
		return errors.Join(err, client.Close())
	}
	if err := client.Quit(); err != nil {
		return errors.Join(fmt.Errorf("close SMTP session: %w", err), client.Close())
	}
	return nil
}

func renderSMTPMessage(message Message) ([]byte, error) {
	if err := message.validate(); err != nil {
		return nil, fmt.Errorf("validate email message: %w", err)
	}

	var buffer bytes.Buffer
	writeSMTPHeader(&buffer, "From", message.From)
	writeSMTPHeader(&buffer, "To", message.To)
	writeSMTPHeader(&buffer, "Subject", mime.QEncoding.Encode(utf8Charset, message.Subject))
	writeSMTPHeader(&buffer, "MIME-Version", "1.0")

	if strings.TrimSpace(message.Text) != "" && strings.TrimSpace(message.HTML) != "" {
		writer := multipart.NewWriter(&buffer)
		writeSMTPHeader(&buffer, "Content-Type", fmt.Sprintf("multipart/alternative; boundary=%q", writer.Boundary()))
		buffer.WriteString("\r\n")
		if err := writeSMTPPart(writer, "text/plain", message.Text); err != nil {
			return nil, err
		}
		if err := writeSMTPPart(writer, "text/html", message.HTML); err != nil {
			return nil, err
		}
		if err := writer.Close(); err != nil {
			return nil, fmt.Errorf("close SMTP multipart body: %w", err)
		}
		return buffer.Bytes(), nil
	}

	contentType, body := "text/plain", message.Text
	if strings.TrimSpace(message.HTML) != "" {
		contentType, body = "text/html", message.HTML
	}
	writeSMTPHeader(&buffer, "Content-Type", contentType+"; charset="+utf8Charset)
	writeSMTPHeader(&buffer, "Content-Transfer-Encoding", "8bit")
	buffer.WriteString("\r\n")
	buffer.WriteString(body)
	return buffer.Bytes(), nil
}

func writeSMTPPart(writer *multipart.Writer, contentType, body string) error {
	header := make(textproto.MIMEHeader, 2)
	header.Set("Content-Type", contentType+"; charset="+utf8Charset)
	header.Set("Content-Transfer-Encoding", "8bit")
	part, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("create SMTP MIME part: %w", err)
	}
	if _, err := part.Write([]byte(body)); err != nil {
		return fmt.Errorf("write SMTP MIME part: %w", err)
	}
	return nil
}

func writeSMTPHeader(buffer *bytes.Buffer, name, value string) {
	buffer.WriteString(name)
	buffer.WriteString(": ")
	buffer.WriteString(value)
	buffer.WriteString("\r\n")
}

func (err *smtpConnectionError) Error() string {
	return "connect to SMTP service"
}

func (err *smtpConnectionError) Unwrap() error {
	return err.cause
}

func (err *smtpConnectionError) Component() string {
	return "smtp"
}

func (err *smtpSendError) Error() string {
	return "send email through SMTP"
}

func (err *smtpSendError) Unwrap() error {
	return err.cause
}

func (err *smtpSendError) Is(target error) bool {
	return target == ErrDeliveryUnavailable
}

func (err *smtpSendError) Component() string {
	return "smtp"
}
