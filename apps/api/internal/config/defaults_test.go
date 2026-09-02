package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfigurationAllowsOAuthCallbackToFinish(t *testing.T) {
	directory := t.TempDir()
	override := filepath.Join(directory, overrideFileName)
	writeFile(t, override, testDevelopmentOverrideYAML)
	paths := configPaths{
		Default:  filepath.Join("..", "..", configDirectoryName, defaultFileName),
		Override: override,
	}

	configuration, err := load(paths, serviceEnvironment)
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if configuration.HTTP.WriteTimeout != 50*time.Second {
		t.Fatalf("HTTP write timeout = %s, want 50s", configuration.HTTP.WriteTimeout)
	}
}
