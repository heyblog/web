package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBackupCOSDisabledNeedsNoCredentials(t *testing.T) {
	// Given / When
	configuration, err := resolveBackupCOS(fileBackupCOSConfig{}, func(string) string { return "" })
	// Then
	require.NoError(t, err)
	require.False(t, configuration.Enabled)
}

func TestBackupCOSRequiresDedicatedValidatedStorage(t *testing.T) {
	for _, test := range []struct {
		name   string
		values fileBackupCOSConfig
	}{
		{"bucket host injection", fileBackupCOSConfig{Enabled: true, Bucket: "bucket.example/", Region: "ap-singapore"}},
		{"region host injection", fileBackupCOSConfig{Enabled: true, Bucket: "backup-123", Region: "ap-singapore.example"}},
		{"prefix traversal", fileBackupCOSConfig{Enabled: true, Bucket: "backup-123", Region: "ap-singapore", Prefix: "../"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given / When
			_, err := resolveBackupCOS(test.values, func(string) string { return "synthetic" })
			// Then
			require.Error(t, err)
		})
	}
}

func TestBackupCOSDefaultsUseDedicatedPrefix(t *testing.T) {
	// Given / When
	configuration, err := resolveBackupCOS(fileBackupCOSConfig{Enabled: true, Bucket: "backup-123", Region: "ap-singapore"}, func(string) string { return "synthetic" })
	// Then
	require.NoError(t, err)
	require.Equal(t, "heyblog-database-uploads/", configuration.Prefix)
}
