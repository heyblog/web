package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type AIConfig struct {
	BaseURL         string
	APIKey          string
	DefaultModel    string
	Timeout         time.Duration
	MaxOutputTokens int64
	ModelsCacheTTL  time.Duration
	Limits          AILimits
	Batch           AIBatchConfig
}

type AIBatchConfig struct {
	Size            int   `yaml:"size"`
	MaxTags         int   `yaml:"max_tags"`
	MaxOutputTokens int64 `yaml:"max_output_tokens"`
}

type AILimits struct {
	UserPerMinute int `yaml:"user_per_minute"`
	IPPerMinute   int `yaml:"ip_per_minute"`
	UserPerDay    int `yaml:"user_per_day"`
	GlobalPerDay  int `yaml:"global_per_day"`
	Concurrent    int `yaml:"concurrent"`
}

type fileAIConfig struct {
	BaseURL         string        `yaml:"base_url"`
	DefaultModel    string        `yaml:"default_model"`
	Timeout         durationValue `yaml:"timeout"`
	MaxOutputTokens int64         `yaml:"max_output_tokens"`
	ModelsCacheTTL  durationValue `yaml:"models_cache_ttl"`
	Limits          AILimits      `yaml:"limits"`
	Batch           AIBatchConfig `yaml:"batch"`
}

func resolveAIConfig(values fileAIConfig, getenv getenvFunc) (AIConfig, error) {
	endpoint, err := url.Parse(values.BaseURL)
	if err != nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return AIConfig{}, fmt.Errorf("ai.base_url must be an HTTPS endpoint without credentials, query or fragment")
	}
	if values.DefaultModel != "deepseek/deepseek-flash" && values.DefaultModel != "deepseek-v4-flash" {
		return AIConfig{}, fmt.Errorf("ai.default_model must be a supported slug-generation model")
	}
	if values.Timeout <= 0 || time.Duration(values.Timeout) > 15*time.Second || values.ModelsCacheTTL <= 0 || values.MaxOutputTokens < 1 || values.MaxOutputTokens > 128 {
		return AIConfig{}, fmt.Errorf("ai timeout must be at most 15s, output tokens 1..128, and cache duration positive")
	}
	limits := values.Limits
	if limits.UserPerMinute < 1 || limits.IPPerMinute < 1 || limits.UserPerDay < 1 || limits.GlobalPerDay < 1 || limits.Concurrent < 1 {
		return AIConfig{}, fmt.Errorf("ai limits must be positive")
	}
	batch := values.Batch
	if batch.Size == 0 && batch.MaxTags == 0 && batch.MaxOutputTokens == 0 {
		batch = AIBatchConfig{Size: 10, MaxTags: 500, MaxOutputTokens: 4096}
	}
	if batch.Size < 1 || batch.Size > 10 || batch.MaxTags < 1 || batch.MaxTags > 500 || batch.MaxOutputTokens < 1 || batch.MaxOutputTokens > 4096 {
		return AIConfig{}, fmt.Errorf("ai batch size must be 1..10, max tags 1..500, and output tokens 1..4096")
	}
	key := strings.TrimSpace(getenv("API_TOKENHUB_API_KEY"))
	if strings.ContainsAny(key, " \t\r\n") {
		return AIConfig{}, fmt.Errorf("API_TOKENHUB_API_KEY must not contain whitespace")
	}
	return AIConfig{BaseURL: values.BaseURL, APIKey: key, DefaultModel: values.DefaultModel,
		Timeout: time.Duration(values.Timeout), MaxOutputTokens: values.MaxOutputTokens, Batch: batch,
		ModelsCacheTTL: time.Duration(values.ModelsCacheTTL), Limits: limits}, nil
}
