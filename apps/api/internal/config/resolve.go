package config

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
)

func resolve(values fileConfig, getenv getenvFunc) (Config, error) {
	if values.Database.MaxConnections < math.MinInt32 || values.Database.MaxConnections > math.MaxInt32 ||
		values.Database.MinConnections < math.MinInt32 || values.Database.MinConnections > math.MaxInt32 {
		return Config{}, fmt.Errorf("database connection bounds exceed int32")
	}

	migrationURL, err := externalURL(getenv, "API_MIGRATION_DATABASE_URL", "postgres", "postgresql")
	if err != nil {
		return Config{}, err
	}
	databaseURL, err := externalURL(getenv, "API_DATABASE_URL", "postgres", "postgresql")
	if err != nil {
		return Config{}, err
	}
	redisURL, err := externalURL(getenv, "API_REDIS_URL", "redis", "rediss", "unix")
	if err != nil {
		return Config{}, err
	}
	healthcheckToken, err := resolveHealthcheckToken(getenv)
	if err != nil {
		return Config{}, err
	}
	webToken, err := resolveBearerToken(getenv, "API_WEB_TOKEN")
	if err != nil {
		return Config{}, err
	}
	tempImportToken, err := resolveBearerToken(getenv, "API_TEMP_IMPORT_TOKEN")
	if err != nil {
		return Config{}, err
	}
	host, err := resolveHost(values.Mode, values.Server.Host)
	if err != nil {
		return Config{}, err
	}
	mailConfiguration, err := resolveMailConfig(values.Mode, values.Mail, getenv)
	if err != nil {
		return Config{}, err
	}
	consoleFormat, err := resolveConsoleFormat(values.Mode, values.Logging.ConsoleFormat)
	if err != nil {
		return Config{}, err
	}
	fileEnabled, err := resolveFileMode(values.Mode, values.Logging.File.Mode)
	if err != nil {
		return Config{}, err
	}

	configuration := Config{
		Mode:                 values.Mode,
		MigrationDatabaseURL: migrationURL,
		HealthcheckToken:     healthcheckToken,
		WebToken:             webToken,
		TempImportToken:      tempImportToken,
		Server:               ServerConfig{Host: host, Port: values.Server.Port},
		Database: DatabaseConfig{
			URL:                   databaseURL,
			MaxConnections:        int32(values.Database.MaxConnections),
			MinConnections:        int32(values.Database.MinConnections),
			MaxConnectionLifetime: time.Duration(values.Database.MaxConnectionLifetime),
			MaxConnectionIdleTime: time.Duration(values.Database.MaxConnectionIdleTime),
			HealthCheckPeriod:     time.Duration(values.Database.HealthCheckPeriod),
		},
		Redis: RedisConfig{
			URL:          redisURL,
			DialTimeout:  time.Duration(values.Redis.DialTimeout),
			ReadTimeout:  time.Duration(values.Redis.ReadTimeout),
			WriteTimeout: time.Duration(values.Redis.WriteTimeout),
		},
		Mail: mailConfiguration,
		Logging: LoggingConfig{
			Level:         strings.ToLower(strings.TrimSpace(values.Logging.Level)),
			ConsoleFormat: consoleFormat,
			File: FileLoggingConfig{
				Enabled:    fileEnabled,
				Path:       resolveFilePath(values.Mode, values.Logging.File.Path),
				MaxSizeMB:  values.Logging.File.MaxSizeMB,
				MaxBackups: values.Logging.File.MaxBackups,
				MaxAgeDays: values.Logging.File.MaxAgeDays,
				Compress:   values.Logging.File.Compress,
			},
		},
		HTTP: HTTPConfig{
			ReadHeaderTimeout: time.Duration(values.HTTP.ReadHeaderTimeout),
			ReadTimeout:       time.Duration(values.HTTP.ReadTimeout),
			WriteTimeout:      time.Duration(values.HTTP.WriteTimeout),
			IdleTimeout:       time.Duration(values.HTTP.IdleTimeout),
			ShutdownTimeout:   time.Duration(values.HTTP.ShutdownTimeout),
			MaxHeaderBytes:    values.HTTP.MaxHeaderBytes,
			MaxBodyBytes:      values.HTTP.MaxBodyBytes,
			TrustedProxies:    slices.Clone(values.HTTP.TrustedProxies),
			CORS: CORSConfig{
				AllowOrigins:     slices.Clone(values.HTTP.CORS.AllowOrigins),
				AllowCredentials: values.HTTP.CORS.AllowCredentials,
			},
		},
		Health: HealthConfig{
			ReadinessTimeout: time.Duration(values.Health.ReadinessTimeout),
			DrainDelay:       time.Duration(values.Health.DrainDelay),
		},
		Auth: resolveAuthConfig(values.Mode, values.Auth, getenv),
	}
	if err := configuration.validate(); err != nil {
		return Config{}, err
	}
	return configuration, nil
}

func (configuration Config) validate() error {
	if configuration.Mode != ModeDevelopment && configuration.Mode != ModeProduction {
		return fmt.Errorf("mode must be development or production")
	}
	if configuration.Server.Port < 1 || configuration.Server.Port > 65535 {
		return fmt.Errorf("server.port must be from 1 to 65535")
	}
	if configuration.Mode == ModeProduction && configuration.Server.Port != productionPort {
		return fmt.Errorf("server.port must be %d in production", productionPort)
	}
	if strings.ContainsAny(configuration.Server.Host, " \t\r\n") {
		return fmt.Errorf("server.host must not contain whitespace")
	}
	if configuration.Database.MaxConnections < 1 || configuration.Database.MinConnections < 0 || configuration.Database.MinConnections > configuration.Database.MaxConnections {
		return fmt.Errorf("database connection bounds are invalid")
	}
	if configuration.Database.MaxConnectionLifetime <= 0 || configuration.Database.MaxConnectionIdleTime <= 0 || configuration.Database.HealthCheckPeriod <= 0 {
		return fmt.Errorf("database durations must be positive")
	}
	if configuration.Redis.DialTimeout <= 0 || configuration.Redis.ReadTimeout <= 0 || configuration.Redis.WriteTimeout <= 0 {
		return fmt.Errorf("redis durations must be positive")
	}
	if err := validateMailConfig(configuration.Mode, configuration.Mail); err != nil {
		return err
	}
	if !slices.Contains([]string{"debug", "info", "warn", "error"}, configuration.Logging.Level) {
		return fmt.Errorf("logging.level must be debug, info, warn, or error")
	}
	if configuration.Logging.File.Path == "" {
		return fmt.Errorf("logging.file.path is required")
	}
	if configuration.Logging.File.MaxSizeMB < 1 || configuration.Logging.File.MaxBackups < 1 || configuration.Logging.File.MaxAgeDays < 1 {
		return fmt.Errorf("logging file rotation values must be positive")
	}
	if configuration.HTTP.ReadHeaderTimeout <= 0 || configuration.HTTP.ReadTimeout <= 0 || configuration.HTTP.WriteTimeout <= 0 || configuration.HTTP.IdleTimeout <= 0 || configuration.HTTP.ShutdownTimeout <= 0 {
		return fmt.Errorf("http timeouts must be positive")
	}
	if configuration.HTTP.MaxHeaderBytes < 1 || configuration.HTTP.MaxBodyBytes < 1 {
		return fmt.Errorf("http size limits must be positive")
	}
	if configuration.Health.ReadinessTimeout <= 0 {
		return fmt.Errorf("health.readiness_timeout must be positive")
	}
	if configuration.Health.DrainDelay < 0 {
		return fmt.Errorf("health.drain_delay must not be negative")
	}
	if err := validateAuthConfig(configuration.Mode, configuration.Auth); err != nil {
		return err
	}
	if err := validateTrustedProxies(configuration.HTTP.TrustedProxies); err != nil {
		return err
	}
	return validateCORS(configuration.HTTP.CORS)
}
