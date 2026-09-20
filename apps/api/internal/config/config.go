package config

import (
	"net"
	"strconv"
	"time"
)

const (
	configVersion       = 1
	productionPort      = 10201
	configDirectoryName = "config"
	defaultFileName     = "default.yaml"
	overrideFileName    = "conf.yaml"
)

type Mode string

const (
	ModeDevelopment Mode = "development"
	ModeProduction  Mode = "production"

	LogFormatText = "text"
	LogFormatJSON = "json"
)

type Config struct {
	Mode                 Mode
	MigrationDatabaseURL string
	HealthcheckToken     string
	WebToken             string
	TempImportToken      string
	Server               ServerConfig
	Database             DatabaseConfig
	Redis                RedisConfig
	Mail                 MailConfig
	Logging              LoggingConfig
	HTTP                 HTTPConfig
	Health               HealthConfig
	Auth                 AuthConfig
}

type ServerConfig struct {
	Host string
	Port int
}

type DatabaseConfig struct {
	URL                   string
	MaxConnections        int32
	MinConnections        int32
	MaxConnectionLifetime time.Duration
	MaxConnectionIdleTime time.Duration
	HealthCheckPeriod     time.Duration
}

type RedisConfig struct {
	URL          string
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type LoggingConfig struct {
	Level         string
	ConsoleFormat string
	File          FileLoggingConfig
}

type FileLoggingConfig struct {
	Enabled    bool
	Path       string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

type HTTPConfig struct {
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	MaxHeaderBytes    int
	MaxBodyBytes      int64
	TrustedProxies    []string
	CORS              CORSConfig
}

type CORSConfig struct {
	AllowOrigins     []string
	AllowCredentials bool
}

type HealthConfig struct {
	ReadinessTimeout time.Duration
	DrainDelay       time.Duration
}

type fileConfig struct {
	Mode     Mode               `yaml:"mode"`
	Version  int                `yaml:"version"`
	Server   fileServerConfig   `yaml:"server"`
	Database fileDatabaseConfig `yaml:"database"`
	Redis    fileRedisConfig    `yaml:"redis"`
	Mail     fileMailConfig     `yaml:"mail"`
	Logging  fileLoggingConfig  `yaml:"logging"`
	HTTP     fileHTTPConfig     `yaml:"http"`
	Health   fileHealthConfig   `yaml:"health"`
	Auth     fileAuthConfig     `yaml:"auth"`
}

type fileServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type fileDatabaseConfig struct {
	MaxConnections        int           `yaml:"max_connections"`
	MinConnections        int           `yaml:"min_connections"`
	MaxConnectionLifetime durationValue `yaml:"max_connection_lifetime"`
	MaxConnectionIdleTime durationValue `yaml:"max_connection_idle_time"`
	HealthCheckPeriod     durationValue `yaml:"health_check_period"`
}

type fileRedisConfig struct {
	DialTimeout  durationValue `yaml:"dial_timeout"`
	ReadTimeout  durationValue `yaml:"read_timeout"`
	WriteTimeout durationValue `yaml:"write_timeout"`
}

type fileLoggingConfig struct {
	Level         string                `yaml:"level"`
	ConsoleFormat string                `yaml:"console_format"`
	File          fileFileLoggingConfig `yaml:"file"`
}

type fileFileLoggingConfig struct {
	Mode       string `yaml:"mode"`
	Path       string `yaml:"path"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAgeDays int    `yaml:"max_age_days"`
	Compress   bool   `yaml:"compress"`
}

type fileHTTPConfig struct {
	ReadHeaderTimeout durationValue  `yaml:"read_header_timeout"`
	ReadTimeout       durationValue  `yaml:"read_timeout"`
	WriteTimeout      durationValue  `yaml:"write_timeout"`
	IdleTimeout       durationValue  `yaml:"idle_timeout"`
	ShutdownTimeout   durationValue  `yaml:"shutdown_timeout"`
	MaxHeaderBytes    int            `yaml:"max_header_bytes"`
	MaxBodyBytes      int64          `yaml:"max_body_bytes"`
	TrustedProxies    []string       `yaml:"trusted_proxies"`
	CORS              fileCORSConfig `yaml:"cors"`
}

type fileCORSConfig struct {
	AllowOrigins     []string `yaml:"allow_origins"`
	AllowCredentials bool     `yaml:"allow_credentials"`
}

type fileHealthConfig struct {
	ReadinessTimeout durationValue `yaml:"readiness_timeout"`
	DrainDelay       durationValue `yaml:"drain_delay"`
}

type durationValue time.Duration

type getenvFunc func(string) string

type configPaths struct {
	Default  string
	Override string
}

func (configuration Config) ListenAddress() string {
	return net.JoinHostPort(configuration.Server.Host, strconv.Itoa(configuration.Server.Port))
}
