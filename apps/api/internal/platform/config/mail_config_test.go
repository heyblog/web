package config

import (
	"strings"
	"testing"
)

func TestLoadAllowsExplicitSESInDevelopmentWithoutSMTPBinding(t *testing.T) {
	t.Parallel()

	getenv := func(key string) string {
		if key == "API_MAIL_SMTP_URL" {
			return ""
		}
		return serviceEnvironment(key)
	}
	paths := writeConfigPair(t, testDefaultYAML, "mode: development\nmail:\n  transport: ses\n")
	got, err := load(paths, getenv)
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if got.Mail.Transport != MailTransportSES {
		t.Fatalf("Mail.Transport = %q, want explicit SES", got.Mail.Transport)
	}
}

func TestLoadRejectsSMTPInProduction(t *testing.T) {
	t.Parallel()

	paths := writeConfigPair(t, testDefaultYAML, "mode: production\nmail:\n  transport: smtp\nauth:\n  web_base_url: https://www.heyblog.net\n")
	_, err := load(paths, serviceEnvironment)
	if err == nil || !strings.Contains(err.Error(), "mail.transport") {
		t.Fatalf("load() error = %v, want production SMTP policy error", err)
	}
}

//nolint:gosec // The credential-like URL is an inert fixture that verifies credential rejection.
func TestLoadRequiresValidSMTPBindingOnlyForSMTPTransport(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"missing":            "",
		"missing port":       "smtp://example.test",
		"credentials":        "smtp://user:secret@example.test:1025",
		"path":               "smtp://example.test:1025/inbox",
		"unsupported scheme": "smtps://example.test:1025",
	}
	for name, smtpURL := range tests {
		t.Run(name, func(t *testing.T) {
			getenv := func(key string) string {
				if key == "API_MAIL_SMTP_URL" {
					return smtpURL
				}
				return serviceEnvironment(key)
			}
			_, err := load(writeConfigPair(t, testDefaultYAML, testDevelopmentOverrideYAML), getenv)
			if err == nil || !strings.Contains(err.Error(), "API_MAIL_SMTP_URL") {
				t.Fatalf("load() error = %v, want SMTP binding validation error", err)
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatalf("load() error leaked SMTP credentials: %v", err)
			}
		})
	}
}

func TestLoadRejectsInvalidMailConfiguration(t *testing.T) {
	t.Parallel()

	tests := map[string]struct{ old, new string }{
		"missing region": {
			old: "region: ap-southeast-1",
			new: "region: ''",
		},
		"region with whitespace": {
			old: "region: ap-southeast-1",
			new: "region: ap southeast 1",
		},
		"region with uppercase": {
			old: "region: ap-southeast-1",
			new: "region: AP-SOUTHEAST-1",
		},
		"region with underscores": {
			old: "region: ap-southeast-1",
			new: "region: ap_southeast_1",
		},
		"region without numeric suffix": {
			old: "region: ap-southeast-1",
			new: "region: not-a-region",
		},
		"region with control character": {
			old: "region: ap-southeast-1",
			new: "region: \"ap-southeast-1\\u0007\"",
		},
		"invalid verification sender": {
			old: "address: no-reply@verify.mail.heyblog.net",
			new: "address: not-an-email",
		},
		"verification sender display name": {
			old: "address: no-reply@verify.mail.heyblog.net",
			new: "address: 'HeyBlog <no-reply@verify.mail.heyblog.net>'",
		},
	}
	for name, replacement := range tests {
		t.Run(name, func(t *testing.T) {
			invalidDefault := strings.Replace(testDefaultYAML, replacement.old, replacement.new, 1)
			_, err := load(writeConfigPair(t, invalidDefault, testDevelopmentOverrideYAML), serviceEnvironment)
			if err == nil {
				t.Fatal("load() error = nil, want invalid mail configuration error")
			}
		})
	}
}

func TestValidateAWSRegionAllowsCurrentPartitionShapes(t *testing.T) {
	t.Parallel()

	for _, region := range []string{"ap-southeast-1", "us-gov-west-1", "eusc-de-east-1"} {
		if err := validateAWSRegion(region); err != nil {
			t.Errorf("validateAWSRegion(%q) error = %v", region, err)
		}
	}
}
