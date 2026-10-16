package config

import (
	"fmt"
	"regexp"
	"strings"
)

// BackupCOSConfig belongs to the API. Credentials never enter Web settings.
type BackupCOSConfig struct {
	Enabled                bool
	Bucket, Region, Prefix string
	SecretID, SecretKey    string
}

type fileBackupCOSConfig struct {
	Enabled bool   `yaml:"enabled"`
	Bucket  string `yaml:"bucket"`
	Region  string `yaml:"region"`
	Prefix  string `yaml:"prefix"`
}

func resolveBackupCOS(values fileBackupCOSConfig, getenv getenvFunc) (BackupCOSConfig, error) {
	configuration := BackupCOSConfig{Enabled: values.Enabled, Bucket: values.Bucket, Region: values.Region,
		Prefix:   values.Prefix,
		SecretID: strings.TrimSpace(getenv("API_BACKUP_COS_SECRET_ID")), SecretKey: strings.TrimSpace(getenv("API_BACKUP_COS_SECRET_KEY"))}
	if !configuration.Enabled {
		return configuration, nil
	}
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]*-[0-9]+$`).MatchString(configuration.Bucket) ||
		!regexp.MustCompile(`^[a-z][a-z0-9-]+$`).MatchString(configuration.Region) {
		return BackupCOSConfig{}, fmt.Errorf("backup_cos bucket or region is invalid")
	}
	if configuration.Prefix == "" {
		configuration.Prefix = "heyblog-database-uploads/"
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9/_-]*/$`).MatchString(configuration.Prefix) || strings.Contains(configuration.Prefix, "//") {
		return BackupCOSConfig{}, fmt.Errorf("backup_cos.prefix must be a dedicated object prefix ending in /")
	}
	if configuration.SecretID == "" || configuration.SecretKey == "" {
		return BackupCOSConfig{}, fmt.Errorf("backup COS credentials are required when enabled")
	}
	return configuration, nil
}
