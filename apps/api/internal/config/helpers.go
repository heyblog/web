package config

import (
	"fmt"
	"net"
	"net/url"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func validateTrustedProxies(proxies []string) error {
	for _, proxy := range proxies {
		if ip := net.ParseIP(proxy); ip != nil {
			if ip.IsUnspecified() {
				return fmt.Errorf("http.trusted_proxies must not trust an unspecified address")
			}
			continue
		}
		_, network, err := net.ParseCIDR(proxy)
		if err != nil {
			return fmt.Errorf("http.trusted_proxies contains an invalid IP or CIDR")
		}
		ones, _ := network.Mask.Size()
		if ones == 0 {
			return fmt.Errorf("http.trusted_proxies must not trust every address")
		}
	}
	return nil
}

func validateCORS(cors CORSConfig) error {
	for _, origin := range cors.AllowOrigins {
		parsed, err := url.Parse(origin)
		if err != nil ||
			(parsed.Scheme != "http" && parsed.Scheme != "https") ||
			parsed.Host == "" || parsed.User != nil || parsed.Path != "" ||
			parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
			return fmt.Errorf("http.cors.allow_origins contains an invalid origin")
		}
	}
	return nil
}

func resolveHost(mode Mode, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "auto" {
		if mode == ModeProduction {
			return "0.0.0.0", nil
		}
		return "127.0.0.1", nil
	}
	if value == "" {
		return "", fmt.Errorf("server.host is required")
	}
	return value, nil
}

func resolveConsoleFormat(mode Mode, value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "auto" {
		if mode == ModeProduction {
			return LogFormatJSON, nil
		}
		return LogFormatText, nil
	}
	if value != LogFormatText && value != LogFormatJSON {
		return "", fmt.Errorf("logging.console_format must be auto, text, or json")
	}
	return value, nil
}

func resolveFileMode(mode Mode, value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "auto":
		return mode == ModeProduction, nil
	case "enabled":
		return true, nil
	case "disabled":
		return false, nil
	default:
		return false, fmt.Errorf("logging.file.mode must be auto, enabled, or disabled")
	}
}

func resolveFilePath(mode Mode, value string) string {
	value = strings.TrimSpace(value)
	if value != "auto" {
		return value
	}
	if mode == ModeProduction {
		return "/var/log/heyblog/api/api.log"
	}
	return "./var/log/heyblog-api.log"
}

func externalURL(getenv getenvFunc, key string, schemes ...string) (string, error) {
	value := strings.TrimSpace(getenv(key))
	if value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	parsed, err := url.Parse(value)
	if err != nil || !slices.Contains(schemes, strings.ToLower(parsed.Scheme)) {
		return "", fmt.Errorf("%s must be a valid service URL", key)
	}
	if parsed.Scheme == "unix" {
		if parsed.Path == "" {
			return "", fmt.Errorf("%s must be a valid service URL", key)
		}
	} else if parsed.Host == "" {
		return "", fmt.Errorf("%s must be a valid service URL", key)
	}
	return value, nil
}

func resolveHealthcheckToken(getenv getenvFunc) (string, error) {
	return resolveBearerToken(getenv, "API_HEALTHCHECK_TOKEN")
}

func resolveBearerToken(getenv getenvFunc, key string) (string, error) {
	const minimumLength = 32

	value := getenv(key)
	if len(value) < minimumLength {
		return "", fmt.Errorf("%s must contain at least %d valid Bearer token characters", key, minimumLength)
	}
	padding := false
	hasTokenCharacter := false
	for index := range len(value) {
		character := value[index]
		if character == '=' {
			padding = true
			continue
		}
		if padding || !isBearerTokenCharacter(character) {
			return "", fmt.Errorf("%s must contain only valid Bearer token characters", key)
		}
		hasTokenCharacter = true
	}
	if !hasTokenCharacter {
		return "", fmt.Errorf("%s must contain a non-padding Bearer token character", key)
	}
	return value, nil
}

func isBearerTokenCharacter(character byte) bool {
	return character >= '0' && character <= '9' ||
		character >= 'A' && character <= 'Z' ||
		character >= 'a' && character <= 'z' ||
		strings.ContainsRune("-._~+/", rune(character))
}

func (duration *durationValue) UnmarshalYAML(node *yaml.Node) error {
	parsed, err := time.ParseDuration(strings.TrimSpace(node.Value))
	if err != nil {
		return fmt.Errorf("must be a Go duration: %w", err)
	}
	*duration = durationValue(parsed)
	return nil
}

func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

func displayPath(path string) string {
	if path == "" {
		return "configuration"
	}
	return path
}
