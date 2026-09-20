package config

import (
	"strings"
	"testing"
)

func TestLoadRejectsNonStandardProductionPort(t *testing.T) {
	t.Parallel()

	paths := writeConfigPair(t, testDefaultYAML, "mode: production\nserver:\n  port: 10300\n")
	if _, err := load(paths, serviceEnvironment); err == nil || !strings.Contains(err.Error(), "10201") {
		t.Fatalf("load() error = %v, want production port policy error", err)
	}
}

func TestLoadRejectsInsecureProductionWebOrigin(t *testing.T) {
	t.Parallel()

	paths := writeConfigPair(t, testDefaultYAML, "mode: production\nauth:\n  web_base_url: http://127.0.0.1:9101\n")
	if _, err := load(paths, serviceEnvironment); err == nil || !strings.Contains(err.Error(), "auth.web_base_url") {
		t.Fatalf("load() error = %v, want production Web origin validation error", err)
	}
}

func TestLoadRejectsWebOriginWithPath(t *testing.T) {
	t.Parallel()

	paths := writeConfigPair(t, testDefaultYAML, "mode: development\nauth:\n  web_base_url: http://127.0.0.1:10101/app\n")
	if _, err := load(paths, serviceEnvironment); err == nil || !strings.Contains(err.Error(), "auth.web_base_url") {
		t.Fatalf("load() error = %v, want Web origin validation error", err)
	}
}

func TestLoadRejectsDeprecatedGithubCallbackURL(t *testing.T) {
	t.Parallel()

	paths := writeConfigPair(t, testDefaultYAML, "mode: development\nauth:\n  web_base_url: http://127.0.0.1:10101\n  github:\n    callback_url: http://127.0.0.1:10101/auth/github/callback\n")
	if _, err := load(paths, serviceEnvironment); err == nil || !strings.Contains(err.Error(), "callback_url") {
		t.Fatalf("load() error = %v, want deprecated callback field error", err)
	}
}

func TestLoadRejectsUnknownNullAndDuplicateFields(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"unknown":   testDevelopmentOverrideYAML + "server:\n  typo_port: 10300\n",
		"null":      testDevelopmentOverrideYAML + "server:\n  host: null\n",
		"duplicate": testDevelopmentOverrideYAML + "server:\n  port: 10201\n  port: 10300\n",
	}
	for name, override := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := load(writeConfigPair(t, testDefaultYAML, override), serviceEnvironment); err == nil {
				t.Fatal("load() error = nil, want strict configuration error")
			}
		})
	}
}

func TestLoadRejectsInvalidVersion(t *testing.T) {
	t.Parallel()

	invalidDefault := strings.Replace(testDefaultYAML, "version: 1", "version: 2", 1)
	if _, err := load(writeConfigPair(t, invalidDefault, testDevelopmentOverrideYAML), serviceEnvironment); err == nil {
		t.Fatal("load() error = nil, want unsupported version error")
	}
}

func TestLoadRequiresExternalBindings(t *testing.T) {
	t.Parallel()

	getenv := func(key string) string {
		if key == "API_DATABASE_URL" {
			return ""
		}
		return serviceEnvironment(key)
	}
	_, err := load(writeConfigPair(t, testDefaultYAML, testDevelopmentOverrideYAML), getenv)
	if err == nil {
		t.Fatal("load() error = nil, want missing external binding error")
	}
	if !strings.Contains(err.Error(), "API_DATABASE_URL") {
		t.Fatalf("load() error = %v, want missing variable name", err)
	}
}

func TestLoadRejectsInvalidHealthcheckToken(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"missing":      "",
		"too short":    "short-token",
		"whitespace":   "test healthcheck token 0123456789abcdef",
		"invalid char": "test-healthcheck-token-0123456789abcde!",
		"padding only": strings.Repeat("=", 32),
	}
	for name, token := range tests {
		t.Run(name, func(t *testing.T) {
			getenv := func(key string) string {
				if key == "API_HEALTHCHECK_TOKEN" {
					return token
				}
				return serviceEnvironment(key)
			}
			_, err := load(writeConfigPair(t, testDefaultYAML, testDevelopmentOverrideYAML), getenv)
			if err == nil {
				t.Fatal("load() error = nil, want invalid healthcheck token error")
			}
			if token != "" && strings.Contains(err.Error(), token) {
				t.Fatal("load() error leaked healthcheck token")
			}
		})
	}
}

func TestLoadRejectsInvalidWebToken(t *testing.T) {
	t.Parallel()

	getenv := func(key string) string {
		if key == "API_WEB_TOKEN" {
			return "short-token"
		}
		return serviceEnvironment(key)
	}
	_, err := load(writeConfigPair(t, testDefaultYAML, testDevelopmentOverrideYAML), getenv)
	if err == nil || !strings.Contains(err.Error(), "API_WEB_TOKEN") {
		t.Fatalf("load() error = %v, want API_WEB_TOKEN validation error", err)
	}
}

func TestLoadRejectsInvalidTempImportToken(t *testing.T) {
	t.Parallel()

	getenv := func(key string) string {
		if key == "API_TEMP_IMPORT_TOKEN" {
			return "short-token"
		}
		return serviceEnvironment(key)
	}
	_, err := load(writeConfigPair(t, testDefaultYAML, testDevelopmentOverrideYAML), getenv)
	if err == nil || !strings.Contains(err.Error(), "API_TEMP_IMPORT_TOKEN") {
		t.Fatalf("load() error = %v, want API_TEMP_IMPORT_TOKEN validation error", err)
	}
}

func TestLoadRejectsInvalidPolicyBounds(t *testing.T) {
	t.Parallel()

	invalidDefault := strings.Replace(testDefaultYAML, "min_connections: 2", "min_connections: 21", 1)
	if _, err := load(writeConfigPair(t, invalidDefault, testDevelopmentOverrideYAML), serviceEnvironment); err == nil {
		t.Fatal("load() error = nil, want invalid pool bounds error")
	}
}

func TestLoadRejectsUnsafeProxyAndMalformedCORSOrigin(t *testing.T) {
	t.Parallel()

	tests := map[string]struct{ old, new string }{
		"trust every IPv4 proxy": {old: "trusted_proxies: []", new: "trusted_proxies: [0.0.0.0/0]"},
		"origin with query":      {old: "allow_origins: []", new: "allow_origins: [https://example.test?token=unsafe]"},
	}
	for name, replacement := range tests {
		t.Run(name, func(t *testing.T) {
			invalidDefault := strings.Replace(testDefaultYAML, replacement.old, replacement.new, 1)
			if _, err := load(writeConfigPair(t, invalidDefault, testDevelopmentOverrideYAML), serviceEnvironment); err == nil {
				t.Fatal("load() error = nil, want unsafe HTTP configuration error")
			}
		})
	}
}

func TestLoadDoesNotLeakMalformedExternalURL(t *testing.T) {
	t.Parallel()

	secret := "postgres://user:super-secret%zz@example.test/heyblog" // #nosec G101 -- this fixture verifies that errors do not leak credentials.
	getenv := func(key string) string {
		if key == "API_MIGRATION_DATABASE_URL" {
			return secret
		}
		return serviceEnvironment(key)
	}
	_, err := load(writeConfigPair(t, testDefaultYAML, testDevelopmentOverrideYAML), getenv)
	if err == nil {
		t.Fatal("load() error = nil, want malformed external URL error")
	}
	if strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("load() error leaked a secret: %v", err)
	}
}
