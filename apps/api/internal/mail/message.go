package mail

import (
	"context"
	"fmt"
	netmail "net/mail"
	"strings"
)

type Message struct {
	From    string
	To      string
	Subject string
	Text    string
	HTML    string
}

type Sender interface {
	Send(context.Context, Message) error
}

func (message Message) validate() error {
	if err := validateAddress(message.From); err != nil {
		return fmt.Errorf("from address: %w", err)
	}
	if err := validateAddress(message.To); err != nil {
		return fmt.Errorf("to address: %w", err)
	}
	if strings.TrimSpace(message.Subject) == "" {
		return fmt.Errorf("subject is required")
	}
	if strings.ContainsAny(message.Subject, "\r\n") {
		return fmt.Errorf("subject must be a single line")
	}
	if strings.TrimSpace(message.Text) == "" && strings.TrimSpace(message.HTML) == "" {
		return fmt.Errorf("text or HTML body is required")
	}
	return nil
}

func validateAddress(value string) error {
	address, err := netmail.ParseAddress(value)
	if err != nil || address.Name != "" || address.Address != value || !isASCII(value) {
		return fmt.Errorf("must be a mailbox address without a display name")
	}
	return nil
}

func isASCII(value string) bool {
	for index := range len(value) {
		if value[index] > 0x7f {
			return false
		}
	}
	return true
}
