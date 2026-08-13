package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testDefaultYAML = `version: 1
server:
  host: auto
  port: 10201
database:
  max_connections: 20
  min_connections: 2
  max_connection_lifetime: 30m
  max_connection_idle_time: 5m
  health_check_period: 1m
redis:
  dial_timeout: 3s
  read_timeout: 2s
  write_timeout: 2s
logging:
  level: info
  console_format: auto
  file:
    mode: auto
    path: auto
    max_size_mb: 100
    max_backups: 10
    max_age_days: 14
    compress: true
http:
  read_header_timeout: 5s
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 1m
  shutdown_timeout: 10s
  max_header_bytes: 65536
  max_body_bytes: 1048576
  trusted_proxies: []
  cors:
    allow_origins: []
    allow_credentials: true
health:
  readiness_timeout: 2s
  drain_delay: 5s
`

const testHealthcheckToken = "test-healthcheck-token-0123456789abcdef"
const testWebToken = "test-web-service-token-0123456789abcdef"

func TestLoadMergesRequiredOverrideAndExternalBindings(t *testing.T) {
	t.Parallel()

	paths := writeConfigPair(t, testDefaultYAML, `mode: production
logging:
  level: debug
  file:
    mode: disabled
http:
  cors:
    allow_origins:
      - https://example.test
`)
	got, err := load(paths, serviceEnvironment)
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	if got.Mode != ModeProduction || got.ListenAddress() != "0.0.0.0:10201" {
		t.Fatalf("runtime = (%q, %q), want production on 0.0.0.0:10201", got.Mode, got.ListenAddress())
	}
	if got.Logging.ConsoleFormat != LogFormatJSON || got.Logging.File.Enabled {
		t.Fatalf("production logging = (%q, %t), want (json, false)", got.Logging.ConsoleFormat, got.Logging.File.Enabled)
	}
	if len(got.HTTP.CORS.AllowOrigins) != 1 || got.HTTP.CORS.AllowOrigins[0] != "https://example.test" {
		t.Fatalf("AllowOrigins = %v, want explicit replacement", got.HTTP.CORS.AllowOrigins)
	}
	if got.MigrationDatabaseURL != serviceEnvironment("API_MIGRATION_DATABASE_URL") ||
		got.Database.URL != serviceEnvironment("API_DATABASE_URL") ||
		got.Redis.URL != serviceEnvironment("API_REDIS_URL") ||
		got.HealthcheckToken != serviceEnvironment("API_HEALTHCHECK_TOKEN") ||
		got.WebToken != serviceEnvironment("API_WEB_TOKEN") {
		t.Fatal("external service bindings were not loaded from the process environment source")
	}
}

func TestLoadUsesDevelopmentModeAndAllowsPortOverride(t *testing.T) {
	t.Parallel()

	paths := writeConfigPair(t, testDefaultYAML, "mode: development\nserver:\n  port: 10300\n")
	got, err := load(paths, serviceEnvironment)
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	if got.ListenAddress() != "127.0.0.1:10300" {
		t.Fatalf("ListenAddress() = %q, want 127.0.0.1:10300", got.ListenAddress())
	}
	if got.Logging.ConsoleFormat != LogFormatText || got.Logging.File.Enabled {
		t.Fatalf("development logging = (%q, %t), want (text, false)", got.Logging.ConsoleFormat, got.Logging.File.Enabled)
	}
	if got.Database.MaxConnectionLifetime != 30*time.Minute || got.HTTP.ShutdownTimeout != 10*time.Second {
		t.Fatalf("durations were not parsed: database=%s shutdown=%s", got.Database.MaxConnectionLifetime, got.HTTP.ShutdownTimeout)
	}
}

func TestDiscoverPathsPrefersExecutableSiblingConfiguration(t *testing.T) {
	t.Parallel()

	executableDirectory := t.TempDir()
	workingDirectory := t.TempDir()
	executableDefault := filepath.Join(executableDirectory, configDirectoryName, defaultFileName)
	writeFile(t, executableDefault, testDefaultYAML)
	writeFile(t, filepath.Join(workingDirectory, configDirectoryName, defaultFileName), testDefaultYAML)

	got, err := discoverPaths(filepath.Join(executableDirectory, "heyblog-api"), workingDirectory)
	if err != nil {
		t.Fatalf("discoverPaths() error = %v", err)
	}
	wantDirectory := filepath.Join(executableDirectory, configDirectoryName)
	if got.Default != filepath.Join(wantDirectory, defaultFileName) || got.Override != filepath.Join(wantDirectory, overrideFileName) {
		t.Fatalf("paths = %#v, want executable sibling directory %q", got, wantDirectory)
	}
}

func TestDiscoverPathsFallsBackToDevelopmentWorkingDirectory(t *testing.T) {
	t.Parallel()

	executableDirectory := t.TempDir()
	workingDirectory := t.TempDir()
	got, err := discoverPaths(filepath.Join(executableDirectory, "temporary-go-run-binary"), workingDirectory)
	if err != nil {
		t.Fatalf("discoverPaths() error = %v", err)
	}
	wantDirectory := filepath.Join(workingDirectory, configDirectoryName)
	if got.Default != filepath.Join(wantDirectory, defaultFileName) || got.Override != filepath.Join(wantDirectory, overrideFileName) {
		t.Fatalf("paths = %#v, want working directory %q", got, wantDirectory)
	}
}

func TestLoadRequiresOverrideAndExplicitMode(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		defaultYAML  string
		overrideYAML *string
	}{
		"missing override": {defaultYAML: testDefaultYAML},
		"missing mode":     {defaultYAML: testDefaultYAML, overrideYAML: stringPointer("server:\n  port: 10300\n")},
		"invalid mode":     {defaultYAML: testDefaultYAML, overrideYAML: stringPointer("mode: staging\n")},
		"mode in default":  {defaultYAML: "mode: development\n" + testDefaultYAML, overrideYAML: stringPointer("mode: development\n")},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			paths := writeConfigFiles(t, test.defaultYAML, test.overrideYAML)
			if _, err := load(paths, serviceEnvironment); err == nil {
				t.Fatal("load() error = nil, want explicit override mode error")
			}
		})
	}
}

func TestLoadRejectsNonStandardProductionPort(t *testing.T) {
	t.Parallel()

	paths := writeConfigPair(t, testDefaultYAML, "mode: production\nserver:\n  port: 10300\n")
	if _, err := load(paths, serviceEnvironment); err == nil || !strings.Contains(err.Error(), "10201") {
		t.Fatalf("load() error = %v, want production port policy error", err)
	}
}

func TestLoadRejectsUnknownNullAndDuplicateFields(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"unknown":   "mode: development\nserver:\n  typo_port: 10300\n",
		"null":      "mode: development\nserver:\n  host: null\n",
		"duplicate": "mode: development\nserver:\n  port: 10201\n  port: 10300\n",
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
	if _, err := load(writeConfigPair(t, invalidDefault, "mode: development\n"), serviceEnvironment); err == nil {
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
	_, err := load(writeConfigPair(t, testDefaultYAML, "mode: development\n"), getenv)
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
			_, err := load(writeConfigPair(t, testDefaultYAML, "mode: development\n"), getenv)
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
	_, err := load(writeConfigPair(t, testDefaultYAML, "mode: development\n"), getenv)
	if err == nil || !strings.Contains(err.Error(), "API_WEB_TOKEN") {
		t.Fatalf("load() error = %v, want API_WEB_TOKEN validation error", err)
	}
}

func TestLoadRejectsInvalidPolicyBounds(t *testing.T) {
	t.Parallel()

	invalidDefault := strings.Replace(testDefaultYAML, "min_connections: 2", "min_connections: 21", 1)
	if _, err := load(writeConfigPair(t, invalidDefault, "mode: development\n"), serviceEnvironment); err == nil {
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
			if _, err := load(writeConfigPair(t, invalidDefault, "mode: development\n"), serviceEnvironment); err == nil {
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
	_, err := load(writeConfigPair(t, testDefaultYAML, "mode: development\n"), getenv)
	if err == nil {
		t.Fatal("load() error = nil, want malformed external URL error")
	}
	if strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("load() error leaked a secret: %v", err)
	}
}

func writeConfigPair(t *testing.T, defaultYAML, overrideYAML string) configPaths {
	t.Helper()
	return writeConfigFiles(t, defaultYAML, &overrideYAML)
}

func writeConfigFiles(t *testing.T, defaultYAML string, overrideYAML *string) configPaths {
	t.Helper()
	directory := filepath.Join(t.TempDir(), configDirectoryName)
	paths := configPaths{
		Default:  filepath.Join(directory, defaultFileName),
		Override: filepath.Join(directory, overrideFileName),
	}
	writeFile(t, paths.Default, defaultYAML)
	if overrideYAML != nil {
		writeFile(t, paths.Override, *overrideYAML)
	}
	return paths
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func stringPointer(value string) *string { return &value }

func serviceEnvironment(key string) string {
	values := map[string]string{
		"API_MIGRATION_DATABASE_URL": "postgres://migrator@example.test/heyblog",
		"API_DATABASE_URL":           "postgres://runtime@example.test/heyblog",
		"API_REDIS_URL":              "redis://example.test:6379/0",
		"API_HEALTHCHECK_TOKEN":      testHealthcheckToken,
		"API_WEB_TOKEN":              testWebToken,
	}
	return values[key]
}
