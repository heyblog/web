package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL               string
	WorkerID                  string
	Port                      int
	PollInterval              time.Duration
	ScheduleInterval          time.Duration
	ConsumeBatch              int
	ConsumeConcurrency        int
	ScheduleBatch             int
	JobHeartbeatInterval      time.Duration
	JobHeartbeatTimeout       time.Duration
	JobRetryLimit             int
	JobRetryDelay             time.Duration
	SiteCheckBatchConcurrency int
	RSSFetchBatchConcurrency  int
	HealthBacklogMax          int
	HealthNoSuccess           time.Duration
	CloudflareURL             string
	CloudflareToken           string
	CallbackTimeout           time.Duration
	APIBaseURL                string
	APIInternalToken          string
	APIRequestTimeout         time.Duration
	Timezone                  *time.Location
}

func Load() (Config, error) {
	timezone, err := time.LoadLocation(envOrDefault("TZ", "Asia/Shanghai"))
	if err != nil {
		return Config{}, fmt.Errorf("load timezone: %w", err)
	}

	cfg := Config{
		DatabaseURL:               strings.TrimSpace(os.Getenv("DATABASE_URL")),
		WorkerID:                  strings.TrimSpace(os.Getenv("WORKER_ID")),
		Port:                      envInt("WORKER_PORT", 9301),
		PollInterval:              envDurationMS("WORKER_POLL_INTERVAL_MS", 2000),
		ScheduleInterval:          envDurationMS("WORKER_SCHEDULE_INTERVAL_MS", 5000),
		ConsumeBatch:              envInt("WORKER_CONSUME_BATCH", 1),
		ConsumeConcurrency:        envInt("WORKER_CONSUME_CONCURRENCY", 4),
		ScheduleBatch:             envInt("WORKER_SCHEDULE_BATCH", 50),
		JobHeartbeatInterval:      envDurationMS("WORKER_JOB_HEARTBEAT_INTERVAL_MS", 5000),
		JobHeartbeatTimeout:       envDurationMS("WORKER_JOB_HEARTBEAT_TIMEOUT_MS", 45000),
		JobRetryLimit:             envInt("WORKER_JOB_RETRY_LIMIT", 2),
		JobRetryDelay:             envDurationMS("WORKER_JOB_RETRY_DELAY_MS", 5000),
		SiteCheckBatchConcurrency: envInt("WORKER_SITE_CHECK_BATCH_CONCURRENCY", 4),
		RSSFetchBatchConcurrency:  envInt("WORKER_RSS_FETCH_BATCH_CONCURRENCY", 4),
		HealthBacklogMax:          envInt("WORKER_HEALTH_BACKLOG_MAX", 500),
		HealthNoSuccess:           envDurationMS("WORKER_HEALTH_NO_SUCCESS_MS", 1800000),
		CloudflareURL:             strings.TrimSpace(os.Getenv("WORKER_CALLBACK_URL")),
		CloudflareToken:           strings.TrimSpace(os.Getenv("WORKER_CALLBACK_SECRET")),
		CallbackTimeout:           envDurationMS("WORKER_CALLBACK_TIMEOUT_MS", 5000),
		APIBaseURL:                strings.TrimSpace(os.Getenv("WORKER_API_BASE_URL")),
		APIInternalToken:          strings.TrimSpace(os.Getenv("API_INTERNAL_TOKEN")),
		APIRequestTimeout:         envDurationMS("WORKER_API_TIMEOUT_MS", 5000),
		Timezone:                  timezone,
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.WorkerID == "" {
		cfg.WorkerID = fmt.Sprintf("worker-%d", time.Now().Unix())
	}

	if cfg.ConsumeBatch <= 0 {
		cfg.ConsumeBatch = 1
	}

	if cfg.ConsumeConcurrency <= 0 {
		cfg.ConsumeConcurrency = 1
	}

	if cfg.ScheduleBatch <= 0 {
		cfg.ScheduleBatch = 50
	}

	if cfg.JobHeartbeatInterval <= 0 {
		cfg.JobHeartbeatInterval = 5 * time.Second
	}

	if cfg.JobHeartbeatTimeout <= cfg.JobHeartbeatInterval {
		cfg.JobHeartbeatTimeout = cfg.JobHeartbeatInterval * 3
	}

	if cfg.JobRetryLimit < 0 {
		cfg.JobRetryLimit = 0
	}

	if cfg.JobRetryDelay < 0 {
		cfg.JobRetryDelay = 0
	}

	if cfg.SiteCheckBatchConcurrency <= 0 {
		cfg.SiteCheckBatchConcurrency = 1
	}

	if cfg.RSSFetchBatchConcurrency <= 0 {
		cfg.RSSFetchBatchConcurrency = 1
	}

	return cfg, nil
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

func envDurationMS(key string, fallbackMS int) time.Duration {
	return time.Duration(envInt(key, fallbackMS)) * time.Millisecond
}
