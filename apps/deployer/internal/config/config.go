package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL        string
	NotifySecret       string
	RepoDir            string
	ComposeFile        string
	SystemdUnit        string
	DockerNamespace    string
	Host               string
	Port               int
	RulesFile          string
	DockerBin          string
	SystemctlBin       string
	GoBin              string
	BinaryPath         string
	ConfigTargetDir    string
	ConfigSyncCommand  string
	AdminNotifyWebhook string
}

func Load() (Config, error) {
	repoDir := strings.TrimSpace(os.Getenv("DEPLOYER_REPO_DIR"))
	cfg := Config{
		DatabaseURL:        strings.TrimSpace(os.Getenv("DATABASE_URL")),
		NotifySecret:       strings.TrimSpace(os.Getenv("DEPLOYER_NOTIFY_SECRET")),
		RepoDir:            repoDir,
		ComposeFile:        envOrDefault("DEPLOYER_COMPOSE_FILE", filepath.Join(repoDir, "infra/docker/docker-compose.prod.yml")),
		SystemdUnit:        envOrDefault("DEPLOYER_SYSTEMD_UNIT", "zhblogs-deployer"),
		DockerNamespace:    strings.TrimSpace(os.Getenv("DOCKERHUB_NAMESPACE")),
		Host:               envOrDefault("DEPLOYER_HOST", "127.0.0.1"),
		Port:               envInt("DEPLOYER_PORT", 9401),
		RulesFile:          envOrDefault("DEPLOYER_RULES_FILE", "apps/deployer/deploy-rules.json"),
		DockerBin:          envOrDefault("DEPLOYER_DOCKER_BIN", "docker"),
		SystemctlBin:       envOrDefault("DEPLOYER_SYSTEMCTL_BIN", "systemctl"),
		GoBin:              envOrDefault("DEPLOYER_GO_BIN", "go"),
		BinaryPath:         envOrDefault("DEPLOYER_BINARY_PATH", "/srv/zhblogs/bin/zhblogs-deployer"),
		ConfigTargetDir:    envOrDefault("DEPLOYER_CONFIG_TARGET_DIR", filepath.Join(repoDir, "infra")),
		ConfigSyncCommand:  strings.TrimSpace(os.Getenv("DEPLOYER_CONFIG_SYNC_COMMAND")),
		AdminNotifyWebhook: strings.TrimSpace(os.Getenv("DEPLOYER_ADMIN_NOTIFY_WEBHOOK")),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.NotifySecret == "" {
		return Config{}, fmt.Errorf("DEPLOYER_NOTIFY_SECRET is required")
	}
	if cfg.RepoDir == "" {
		return Config{}, fmt.Errorf("DEPLOYER_REPO_DIR is required")
	}
	if cfg.DockerNamespace == "" {
		return Config{}, fmt.Errorf("DOCKERHUB_NAMESPACE is required")
	}
	if !filepath.IsAbs(cfg.RulesFile) {
		cfg.RulesFile = filepath.Join(cfg.RepoDir, cfg.RulesFile)
	}

	return cfg, nil
}

func (cfg Config) Addr() string {
	return fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
