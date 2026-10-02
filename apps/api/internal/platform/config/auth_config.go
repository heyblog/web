package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type AuthConfig struct {
	AccessSecret       string
	RefreshSecret      string
	AccessTTL          time.Duration
	RefreshTTL         time.Duration
	VerificationTTL    time.Duration
	PasswordResetTTL   time.Duration
	WebBaseURL         string
	CookieDomain       string
	GithubClientID     string
	GithubClientSecret string
	GithubScope        string
}

type fileAuthConfig struct {
	WebBaseURL       string           `yaml:"web_base_url"`
	CookieDomain     string           `yaml:"cookie_domain"`
	AccessTTL        durationValue    `yaml:"access_ttl"`
	RefreshTTL       durationValue    `yaml:"refresh_ttl"`
	VerificationTTL  durationValue    `yaml:"verification_ttl"`
	PasswordResetTTL durationValue    `yaml:"password_reset_ttl"`
	Github           fileGithubConfig `yaml:"github"`
}

type fileGithubConfig struct {
	Scope string `yaml:"scope"`
}

func resolveAuthConfig(mode Mode, values fileAuthConfig, getenv getenvFunc) AuthConfig {
	return AuthConfig{
		AccessSecret:       resolveAuthSecret(getenv, "API_AUTH_ACCESS_SECRET", mode),
		RefreshSecret:      resolveAuthSecret(getenv, "API_AUTH_REFRESH_SECRET", mode),
		AccessTTL:          time.Duration(values.AccessTTL),
		RefreshTTL:         time.Duration(values.RefreshTTL),
		VerificationTTL:    time.Duration(values.VerificationTTL),
		PasswordResetTTL:   time.Duration(values.PasswordResetTTL),
		WebBaseURL:         strings.TrimRight(strings.TrimSpace(values.WebBaseURL), "/"),
		CookieDomain:       strings.TrimSpace(values.CookieDomain),
		GithubClientID:     strings.TrimSpace(getenv("API_GITHUB_CLIENT_ID")),
		GithubClientSecret: strings.TrimSpace(getenv("API_GITHUB_CLIENT_SECRET")),
		GithubScope:        strings.TrimSpace(values.Github.Scope),
	}
}

func validateAuthConfig(mode Mode, configuration AuthConfig) error {
	if configuration.AccessTTL <= 0 || configuration.RefreshTTL <= 0 ||
		configuration.VerificationTTL <= 0 || configuration.PasswordResetTTL <= 0 {
		return fmt.Errorf("auth durations must be positive")
	}
	if configuration.GithubScope == "" {
		return fmt.Errorf("auth.github.scope is required")
	}
	if err := validateWebBaseURL(mode, configuration.WebBaseURL); err != nil {
		return err
	}
	if len(configuration.AccessSecret) < 32 || len(configuration.RefreshSecret) < 32 {
		return fmt.Errorf("auth secrets must contain at least 32 characters")
	}
	return nil
}

func validateWebBaseURL(mode Mode, value string) error {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
		parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
		return fmt.Errorf("auth.web_base_url must be an HTTP origin")
	}
	if mode == ModeProduction && parsed.Scheme != "https" {
		return fmt.Errorf("auth.web_base_url must use HTTPS in production")
	}
	return nil
}

func resolveAuthSecret(getenv getenvFunc, key string, mode Mode) string {
	value := strings.TrimSpace(getenv(key))
	if value != "" {
		return value
	}
	if mode == ModeDevelopment {
		return key + "-development-secret-012345678901234567890"
	}
	return ""
}
