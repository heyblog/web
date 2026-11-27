package mail

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/smtp"
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
