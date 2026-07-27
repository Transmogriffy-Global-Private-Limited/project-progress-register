package config

import (
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
)

// Config contains the complete foundation runtime configuration.
type Config struct {
	AppName              string
	Environment          string
	HTTPAddress          string
	DatabaseURL          string
	MigrationDatabaseURL string
	APIDocsEnabled       bool
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
		AppName:          valueOrDefault(lookup, "APP_NAME", defaultAppName),
		Environment:      valueOrDefault(lookup, "APP_ENV", defaultEnvironment),
		HTTPAddress:      valueOrDefault(lookup, "HTTP_ADDR", defaultHTTPAddress),
		ReadinessTimeout: defaultReadinessTimeout,
		ShutdownTimeout:  defaultShutdownTimeout,
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

	var err error
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
