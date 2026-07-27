package config

import (
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAppName          = "Project Progress Register"
	defaultEnvironment      = "development"
	defaultHTTPAddress      = "127.0.0.1:8080"
	defaultReadinessTimeout = 2 * time.Second
	defaultShutdownTimeout  = 10 * time.Second
	defaultSessionTTL       = 12 * time.Hour
	defaultAttachmentRoot   = ".local/attachments"
	defaultAttachmentBytes  = int64(100 << 20)
	defaultAttachmentCount  = 10
)

// Config contains the complete foundation runtime configuration.
type Config struct {
	AppName              string
	Environment          string
	HTTPAddress          string
	DatabaseURL          string
	MigrationDatabaseURL string
	APIDocsEnabled       bool
	SessionCSRFKey       []byte
	SessionTTL           time.Duration
	BootstrapToken       string
	AttachmentStorageDir string
	AttachmentMaxBytes   int64
	AttachmentMaxCount   int
	ReadinessTimeout     time.Duration
	ShutdownTimeout      time.Duration
}

// LookupEnv allows configuration loading to be tested without mutating process state.
type LookupEnv func(string) (string, bool)

// Load reads and validates configuration from process environment variables.
func Load() (Config, error) {
	return LoadWithLookup(os.LookupEnv)
}

// LoadWithLookup reads and validates configuration with the supplied environment lookup.
func LoadWithLookup(lookup LookupEnv) (Config, error) {
	if lookup == nil {
		return Config{}, fmt.Errorf("environment lookup is required")
	}

	cfg := Config{
		AppName:              valueOrDefault(lookup, "APP_NAME", defaultAppName),
		Environment:          valueOrDefault(lookup, "APP_ENV", defaultEnvironment),
		HTTPAddress:          valueOrDefault(lookup, "HTTP_ADDR", defaultHTTPAddress),
		ReadinessTimeout:     defaultReadinessTimeout,
		ShutdownTimeout:      defaultShutdownTimeout,
		SessionTTL:           defaultSessionTTL,
		AttachmentStorageDir: defaultAttachmentRoot,
		AttachmentMaxBytes:   defaultAttachmentBytes,
		AttachmentMaxCount:   defaultAttachmentCount,
	}

	if !isEnvironment(cfg.Environment) {
		return Config{}, fmt.Errorf("APP_ENV must be development, test, or production")
	}
	if err := validateLoopbackAddress(cfg.HTTPAddress); err != nil {
		return Config{}, fmt.Errorf("HTTP_ADDR: %w", err)
	}

	var ok bool
	cfg.DatabaseURL, ok = nonEmptyValue(lookup, "DATABASE_URL")
	if !ok {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	cfg.MigrationDatabaseURL, _ = nonEmptyValue(lookup, "MIGRATION_DATABASE_URL")
	if cfg.MigrationDatabaseURL == "" {
		cfg.MigrationDatabaseURL = cfg.DatabaseURL
	}

	if raw, exists := lookup("API_DOCS_ENABLED"); exists && strings.TrimSpace(raw) != "" {
		value, err := parseStrictBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("API_DOCS_ENABLED: %w", err)
		}
		cfg.APIDocsEnabled = value
	}
	if raw, ok := nonEmptyValue(lookup, "SESSION_CSRF_KEY"); ok {
		key, err := base64.StdEncoding.DecodeString(raw)
		if err != nil || len(key) != 32 {
			return Config{}, fmt.Errorf("SESSION_CSRF_KEY must be standard base64 encoding of exactly 32 bytes")
		}
		cfg.SessionCSRFKey = key
	}
	if raw, ok := nonEmptyValue(lookup, "BOOTSTRAP_TOKEN"); ok {
		if len(raw) < 24 || len(raw) > 256 {
			return Config{}, fmt.Errorf("BOOTSTRAP_TOKEN must contain 24 through 256 characters")
		}
		cfg.BootstrapToken = raw
	}
	if raw, ok := nonEmptyValue(lookup, "ATTACHMENT_STORAGE_DIR"); ok {
		cfg.AttachmentStorageDir = raw
	}
	if raw, ok := nonEmptyValue(lookup, "ATTACHMENT_MAX_FILE_BYTES"); ok {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 1<<20 || value > 1<<30 {
			return Config{}, fmt.Errorf("ATTACHMENT_MAX_FILE_BYTES must be an integer from 1048576 through 1073741824")
		}
		cfg.AttachmentMaxBytes = value
	}
	if raw, ok := nonEmptyValue(lookup, "ATTACHMENT_MAX_FILES_PER_UPDATE"); ok {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 25 {
			return Config{}, fmt.Errorf("ATTACHMENT_MAX_FILES_PER_UPDATE must be an integer from 1 through 25")
		}
		cfg.AttachmentMaxCount = value
	}

	var err error
	cfg.SessionTTL, err = durationValue(lookup, "SESSION_TTL", defaultSessionTTL, 15*time.Minute, 7*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	cfg.ReadinessTimeout, err = durationValue(lookup, "READINESS_TIMEOUT", defaultReadinessTimeout, 100*time.Millisecond, 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	cfg.ShutdownTimeout, err = durationValue(lookup, "SHUTDOWN_TIMEOUT", defaultShutdownTimeout, time.Second, time.Minute)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func valueOrDefault(lookup LookupEnv, key, fallback string) string {
	if value, ok := nonEmptyValue(lookup, key); ok {
		return value
	}
	return fallback
}

func nonEmptyValue(lookup LookupEnv, key string) (string, bool) {
	value, ok := lookup(key)
	value = strings.TrimSpace(value)
	return value, ok && value != ""
}

func isEnvironment(value string) bool {
	switch value {
	case "development", "test", "production":
		return true
	default:
		return false
	}
}

func parseStrictBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("must be true or false")
	}
}

func durationValue(lookup LookupEnv, key string, fallback, minimum, maximum time.Duration) (time.Duration, error) {
	raw, ok := nonEmptyValue(lookup, key)
	if !ok {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid duration: %w", key, err)
	}
	if value < minimum || value > maximum {
		return 0, fmt.Errorf("%s must be between %s and %s", key, minimum, maximum)
	}
	return value, nil
}

func validateLoopbackAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("must be a host:port pair: %w", err)
	}
	if !strings.EqualFold(host, "localhost") {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("host must be a loopback address")
		}
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1024 || port > 65535 {
		return fmt.Errorf("port must be an unprivileged port from 1024 through 65535")
	}
	return nil
}
