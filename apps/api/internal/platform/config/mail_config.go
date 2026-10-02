package config

import (
	"fmt"
	"net"
	netmail "net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type MailTransport string

const (
	MailTransportSMTP MailTransport = "smtp"
	MailTransportSES  MailTransport = "ses"
)

type MailConfig struct {
	Transport MailTransport
	SMTP      SMTPConfig
	SES       SESConfig
	Senders   MailSendersConfig
}

type SMTPConfig struct {
	Address string
	Timeout time.Duration
}

type SESConfig struct {
	Region string
}

type MailSendersConfig struct {
	Verification MailSenderConfig
	Submission   MailSenderConfig
}

type MailSenderConfig struct {
	Address string
}

type fileMailConfig struct {
	Transport string                `yaml:"transport"`
	SMTP      fileSMTPConfig        `yaml:"smtp"`
	SES       fileSESConfig         `yaml:"ses"`
	Senders   fileMailSendersConfig `yaml:"senders"`
}

type fileSMTPConfig struct {
	Timeout durationValue `yaml:"timeout"`
}

type fileSESConfig struct {
	Region string `yaml:"region"`
}

type fileMailSendersConfig struct {
	Verification fileMailSenderConfig `yaml:"verification"`
	Submission   fileMailSenderConfig `yaml:"submission"`
}

type fileMailSenderConfig struct {
	Address string `yaml:"address"`
}

func resolveMailConfig(mode Mode, values fileMailConfig, getenv getenvFunc) (MailConfig, error) {
	transport, err := resolveMailTransport(mode, values.Transport)
	if err != nil {
		return MailConfig{}, err
	}

	smtpAddress := ""
	if transport == MailTransportSMTP {
		smtpAddress, err = resolveSMTPAddress(getenv("API_MAIL_SMTP_URL"))
		if err != nil {
			return MailConfig{}, err
		}
	}

	return MailConfig{
		Transport: transport,
		SMTP: SMTPConfig{
			Address: smtpAddress,
			Timeout: time.Duration(values.SMTP.Timeout),
		},
		SES: SESConfig{Region: strings.TrimSpace(values.SES.Region)},
		Senders: MailSendersConfig{
			Verification: MailSenderConfig{Address: strings.TrimSpace(values.Senders.Verification.Address)},
			Submission:   MailSenderConfig{Address: strings.TrimSpace(values.Senders.Submission.Address)},
		},
	}, nil
}

func resolveMailTransport(mode Mode, value string) (MailTransport, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "auto":
		if mode == ModeProduction {
			return MailTransportSES, nil
		}
		return MailTransportSMTP, nil
	case string(MailTransportSMTP):
		if mode == ModeProduction {
			return "", fmt.Errorf("mail.transport must be ses in production")
		}
		return MailTransportSMTP, nil
	case string(MailTransportSES):
		return MailTransportSES, nil
	default:
		return "", fmt.Errorf("mail.transport must be auto, smtp, or ses")
	}
}

func resolveSMTPAddress(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "smtp" || parsed.Host == "" || parsed.User != nil ||
		parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
		return "", fmt.Errorf("API_MAIL_SMTP_URL must be a valid SMTP service URL")
	}
	host, portValue := parsed.Hostname(), parsed.Port()
	port, portErr := strconv.Atoi(portValue)
	if host == "" || portErr != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("API_MAIL_SMTP_URL must be a valid SMTP service URL")
	}
	return net.JoinHostPort(host, portValue), nil
}

func validateMailConfig(mode Mode, configuration MailConfig) error {
	if configuration.Transport != MailTransportSMTP && configuration.Transport != MailTransportSES {
		return fmt.Errorf("mail transport is invalid")
	}
	if mode == ModeProduction && configuration.Transport != MailTransportSES {
		return fmt.Errorf("mail.transport must be ses in production")
	}
	if configuration.SMTP.Timeout <= 0 {
		return fmt.Errorf("mail.smtp.timeout must be positive")
	}
	if configuration.Transport == MailTransportSMTP && configuration.SMTP.Address == "" {
		return fmt.Errorf("API_MAIL_SMTP_URL is required")
	}
	if err := validateAWSRegion(configuration.SES.Region); err != nil {
		return err
	}
	if err := validateMailbox("mail.senders.verification.address", configuration.Senders.Verification.Address); err != nil {
		return err
	}
	return validateMailbox("mail.senders.submission.address", configuration.Senders.Submission.Address)
}

func validateMailbox(path, value string) error {
	address, err := netmail.ParseAddress(value)
	if err != nil || address.Name != "" || address.Address != value || !isASCII(value) {
		return fmt.Errorf("%s must be a mailbox address without a display name", path)
	}
	return nil
}

func validateAWSRegion(value string) error {
	parts := strings.Split(value, "-")
	if len(parts) < 3 || parts[0] == "" || parts[0][0] < 'a' || parts[0][0] > 'z' {
		return fmt.Errorf("mail.ses.region must use AWS region syntax")
	}
	for _, part := range parts {
		if part == "" {
			return fmt.Errorf("mail.ses.region must use AWS region syntax")
		}
		for _, character := range part {
			if character < 'a' || character > 'z' {
				if character < '0' || character > '9' {
					return fmt.Errorf("mail.ses.region must use AWS region syntax")
				}
			}
		}
	}
	for _, character := range parts[len(parts)-1] {
		if character < '0' || character > '9' {
			return fmt.Errorf("mail.ses.region must use AWS region syntax")
		}
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
