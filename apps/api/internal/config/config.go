package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
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
	Server               ServerConfig
	Database             DatabaseConfig
	Redis                RedisConfig
	Logging              LoggingConfig
	HTTP                 HTTPConfig
	Health               HealthConfig
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
	Logging  fileLoggingConfig  `yaml:"logging"`
	HTTP     fileHTTPConfig     `yaml:"http"`
	Health   fileHealthConfig   `yaml:"health"`
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

func Load() (Config, error) {
	paths, err := locateConfigPaths()
	if err != nil {
		return Config{}, err
	}
	return load(paths, os.Getenv)
}

func locateConfigPaths() (configPaths, error) {
	executable, err := os.Executable()
	if err != nil {
		return configPaths{}, fmt.Errorf("locate API executable: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return configPaths{}, fmt.Errorf("resolve API executable: %w", err)
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return configPaths{}, fmt.Errorf("locate API working directory: %w", err)
	}
	return discoverPaths(executable, workingDirectory)
}

func discoverPaths(executable, workingDirectory string) (configPaths, error) {
	executableDirectory := filepath.Join(filepath.Dir(executable), configDirectoryName)
	executableDefault := filepath.Join(executableDirectory, defaultFileName)
	if _, err := os.Stat(executableDefault); err == nil {
		return pathsIn(executableDirectory), nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return configPaths{}, fmt.Errorf("inspect executable configuration: %w", err)
	}
	return pathsIn(filepath.Join(workingDirectory, configDirectoryName)), nil
}

func pathsIn(directory string) configPaths {
	return configPaths{
		Default:  filepath.Join(directory, defaultFileName),
		Override: filepath.Join(directory, overrideFileName),
	}
}

func load(paths configPaths, getenv getenvFunc) (Config, error) {
	base, err := readYAML(paths.Default)
	if err != nil {
		return Config{}, fmt.Errorf("read default configuration: %w", err)
	}
	if mappingValueIndex(base, "mode") >= 0 {
		return Config{}, fmt.Errorf("default configuration must not define mode")
	}
	override, err := readYAML(paths.Override)
	if err != nil {
		return Config{}, fmt.Errorf("read configuration override: %w", err)
	}
	if mappingValueIndex(override, "mode") < 0 {
		return Config{}, fmt.Errorf("configuration override must define mode")
	}
	mergeYAML(base, override)

	fileValues, err := decodeFileConfig(base)
	if err != nil {
		return Config{}, err
	}
	return resolve(fileValues, getenv)
}

func readYAML(path string) (*yaml.Node, error) {
	contents, err := os.ReadFile(path) // #nosec G304 -- configuration paths are derived from the executable or working directory.
	if err != nil {
		return nil, err
	}

	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("decode %s: multiple YAML documents are not allowed", path)
		}
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("decode %s: root must be a mapping", path)
	}
	if err := validateYAMLNode(document.Content[0], ""); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return document.Content[0], nil
}

func validateYAMLNode(node *yaml.Node, path string) error {
	if node.Kind == yaml.AliasNode {
		return fmt.Errorf("%s: aliases are not allowed", displayPath(path))
	}
	if node.Tag == "!!null" {
		return fmt.Errorf("%s: null values are not allowed", displayPath(path))
	}
	if node.Kind == yaml.MappingNode {
		seen := make(map[string]struct{}, len(node.Content)/2)
		for index := 0; index < len(node.Content); index += 2 {
			key := node.Content[index]
			if key.Kind != yaml.ScalarNode || key.Value == "" {
				return fmt.Errorf("%s: mapping keys must be non-empty strings", displayPath(path))
			}
			if _, exists := seen[key.Value]; exists {
				return fmt.Errorf("%s: duplicate field", joinPath(path, key.Value))
			}
			seen[key.Value] = struct{}{}
			if err := validateYAMLNode(node.Content[index+1], joinPath(path, key.Value)); err != nil {
				return err
			}
		}
		return nil
	}
	for index, child := range node.Content {
		if err := validateYAMLNode(child, joinPath(path, strconv.Itoa(index))); err != nil {
			return err
		}
	}
	return nil
}

func mergeYAML(base, override *yaml.Node) {
	for index := 0; index < len(override.Content); index += 2 {
		key := override.Content[index]
		value := override.Content[index+1]
		baseIndex := mappingValueIndex(base, key.Value)
		if baseIndex < 0 {
			base.Content = append(base.Content, cloneYAMLNode(key), cloneYAMLNode(value))
			continue
		}
		baseValue := base.Content[baseIndex]
		if baseValue.Kind == yaml.MappingNode && value.Kind == yaml.MappingNode {
			mergeYAML(baseValue, value)
			continue
		}
		base.Content[baseIndex] = cloneYAMLNode(value)
	}
}

func mappingValueIndex(mapping *yaml.Node, key string) int {
	for index := 0; index < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return index + 1
		}
	}
	return -1
}

func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	clone := *node
	clone.Content = make([]*yaml.Node, len(node.Content))
	for index, child := range node.Content {
		clone.Content[index] = cloneYAMLNode(child)
	}
	return &clone
}

func decodeFileConfig(root *yaml.Node) (fileConfig, error) {
	document := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
	contents, err := yaml.Marshal(document)
	if err != nil {
		return fileConfig{}, fmt.Errorf("encode merged configuration: %w", err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	decoder.KnownFields(true)
	var values fileConfig
	if err := decoder.Decode(&values); err != nil {
		return fileConfig{}, fmt.Errorf("decode merged configuration: %w", err)
	}
	if values.Version != configVersion {
		return fileConfig{}, fmt.Errorf("configuration version must be %d", configVersion)
	}
	return values, nil
}

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

	host, err := resolveHost(values.Mode, values.Server.Host)
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
	filePath := resolveFilePath(values.Mode, values.Logging.File.Path)

	config := Config{
		Mode:                 values.Mode,
		MigrationDatabaseURL: migrationURL,
		HealthcheckToken:     healthcheckToken,
		WebToken:             webToken,
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
		Logging: LoggingConfig{
			Level:         strings.ToLower(strings.TrimSpace(values.Logging.Level)),
			ConsoleFormat: consoleFormat,
			File: FileLoggingConfig{
				Enabled:    fileEnabled,
				Path:       filePath,
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
	}
	if err := config.validate(); err != nil {
		return Config{}, err
	}
	return config, nil
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
	if err := validateTrustedProxies(configuration.HTTP.TrustedProxies); err != nil {
		return err
	}
	return validateCORS(configuration.HTTP.CORS)
}

func validateTrustedProxies(proxies []string) error {
	for _, proxy := range proxies {
		if ip := net.ParseIP(proxy); ip != nil {
			if ip.IsUnspecified() {
				return fmt.Errorf("http.trusted_proxies must not trust an unspecified address")
			}
			continue
		}
		_, network, err := net.ParseCIDR(proxy)
		if err != nil {
			return fmt.Errorf("http.trusted_proxies contains an invalid IP or CIDR")
		}
		ones, _ := network.Mask.Size()
		if ones == 0 {
			return fmt.Errorf("http.trusted_proxies must not trust every address")
		}
	}
	return nil
}

func validateCORS(cors CORSConfig) error {
	for _, origin := range cors.AllowOrigins {
		parsed, err := url.Parse(origin)
		if err != nil ||
			(parsed.Scheme != "http" && parsed.Scheme != "https") ||
			parsed.Host == "" || parsed.User != nil || parsed.Path != "" ||
			parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
			return fmt.Errorf("http.cors.allow_origins contains an invalid origin")
		}
	}
	return nil
}

func resolveHost(mode Mode, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "auto" {
		if mode == ModeProduction {
			return "0.0.0.0", nil
		}
		return "127.0.0.1", nil
	}
	if value == "" {
		return "", fmt.Errorf("server.host is required")
	}
	return value, nil
}

func resolveConsoleFormat(mode Mode, value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "auto" {
		if mode == ModeProduction {
			return LogFormatJSON, nil
		}
		return LogFormatText, nil
	}
	if value != LogFormatText && value != LogFormatJSON {
		return "", fmt.Errorf("logging.console_format must be auto, text, or json")
	}
	return value, nil
}

func resolveFileMode(mode Mode, value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "auto":
		return mode == ModeProduction, nil
	case "enabled":
		return true, nil
	case "disabled":
		return false, nil
	default:
		return false, fmt.Errorf("logging.file.mode must be auto, enabled, or disabled")
	}
}

func resolveFilePath(mode Mode, value string) string {
	value = strings.TrimSpace(value)
	if value != "auto" {
		return value
	}
	if mode == ModeProduction {
		return "/var/log/heyblog/api/api.log"
	}
	return "./var/log/heyblog-api.log"
}

func externalURL(getenv getenvFunc, key string, schemes ...string) (string, error) {
	value := strings.TrimSpace(getenv(key))
	if value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	parsed, err := url.Parse(value)
	if err != nil || !slices.Contains(schemes, strings.ToLower(parsed.Scheme)) {
		return "", fmt.Errorf("%s must be a valid service URL", key)
	}
	if parsed.Scheme == "unix" {
		if parsed.Path == "" {
			return "", fmt.Errorf("%s must be a valid service URL", key)
		}
	} else if parsed.Host == "" {
		return "", fmt.Errorf("%s must be a valid service URL", key)
	}
	return value, nil
}

func resolveHealthcheckToken(getenv getenvFunc) (string, error) {
	return resolveBearerToken(getenv, "API_HEALTHCHECK_TOKEN")
}

func resolveBearerToken(getenv getenvFunc, key string) (string, error) {
	const minimumLength = 32

	value := getenv(key)
	if len(value) < minimumLength {
		return "", fmt.Errorf("%s must contain at least %d valid Bearer token characters", key, minimumLength)
	}
	padding := false
	hasTokenCharacter := false
	for index := range len(value) {
		character := value[index]
		if character == '=' {
			padding = true
			continue
		}
		if padding || !isBearerTokenCharacter(character) {
			return "", fmt.Errorf("%s must contain only valid Bearer token characters", key)
		}
		hasTokenCharacter = true
	}
	if !hasTokenCharacter {
		return "", fmt.Errorf("%s must contain a non-padding Bearer token character", key)
	}
	return value, nil
}

func isBearerTokenCharacter(character byte) bool {
	return character >= '0' && character <= '9' ||
		character >= 'A' && character <= 'Z' ||
		character >= 'a' && character <= 'z' ||
		strings.ContainsRune("-._~+/", rune(character))
}

func (configuration Config) ListenAddress() string {
	return net.JoinHostPort(configuration.Server.Host, strconv.Itoa(configuration.Server.Port))
}

func (duration *durationValue) UnmarshalYAML(node *yaml.Node) error {
	parsed, err := time.ParseDuration(strings.TrimSpace(node.Value))
	if err != nil {
		return fmt.Errorf("must be a Go duration: %w", err)
	}
	*duration = durationValue(parsed)
	return nil
}

func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

func displayPath(path string) string {
	if path == "" {
		return "configuration"
	}
	return path
}
