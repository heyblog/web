package logging

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"heyblog-api/internal/config"
)

func TestNewWritesDevelopmentTextToConsoleOnly(t *testing.T) {
	t.Parallel()

	var console bytes.Buffer
	runtime, err := New(config.ModeDevelopment, config.LoggingConfig{
		Level:         "info",
		ConsoleFormat: config.LogFormatText,
		File:          config.FileLoggingConfig{Enabled: false},
	}, &console)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Close() })

	runtime.Logger.Info("server started", "event", "server_started")
	output := console.String()
	if !strings.Contains(output, "server started") || !strings.Contains(output, "service=heyblog-api") || !strings.Contains(output, "environment=development") {
		t.Fatalf("console output = %q, want common structured attributes", output)
	}
}

func TestNewWritesProductionJSONToConsoleAndFile(t *testing.T) {
	t.Parallel()

	var console bytes.Buffer
	logPath := filepath.Join(t.TempDir(), "logs", "api.log")
	runtime, err := New(config.ModeProduction, config.LoggingConfig{
		Level:         "info",
		ConsoleFormat: config.LogFormatJSON,
		File: config.FileLoggingConfig{
			Enabled:    true,
			Path:       logPath,
			MaxSizeMB:  100,
			MaxBackups: 10,
			MaxAgeDays: 14,
			Compress:   true,
		},
	}, &console)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	runtime.Logger.Warn("dependency slow", "event", "dependency_slow")
	if err := runtime.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	fileOutput, err := os.ReadFile(logPath) // #nosec G304 -- logPath is a temporary test directory path.
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", logPath, err)
	}

	for name, output := range map[string]string{"console": console.String(), "file": string(fileOutput)} {
		if !strings.Contains(output, `"msg":"dependency slow"`) || !strings.Contains(output, `"environment":"production"`) {
			t.Fatalf("%s output = %q, want JSON log with common attributes", name, output)
		}
	}
}

func TestNewFailsWhenFileCannotBeInitialized(t *testing.T) {
	t.Parallel()

	parent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parent, []byte("file"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := New(config.ModeProduction, config.LoggingConfig{
		Level:         "info",
		ConsoleFormat: config.LogFormatJSON,
		File: config.FileLoggingConfig{
			Enabled:    true,
			Path:       filepath.Join(parent, "api.log"),
			MaxSizeMB:  100,
			MaxBackups: 10,
			MaxAgeDays: 14,
		},
	}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("New() error = nil, want file initialization failure")
	}
}
