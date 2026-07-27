package config

import (
	"testing"
	"time"
)

func TestLoadWithLookupDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := LoadWithLookup(mapLookup(map[string]string{
		"DATABASE_URL": "postgres://example.invalid/ppr",
	}))
	if err != nil {
		t.Fatalf("LoadWithLookup() error = %v", err)
	}
	if cfg.HTTPAddress != "127.0.0.1:8080" {
		t.Fatalf("HTTPAddress = %q", cfg.HTTPAddress)
	}
	if cfg.BasePath != "" {
		t.Fatalf("BasePath = %q", cfg.BasePath)
	}
	if cfg.APIDocsEnabled {
		t.Fatal("APIDocsEnabled should default to false")
	}
	if cfg.ReadinessTimeout != 2*time.Second {
		t.Fatalf("ReadinessTimeout = %s", cfg.ReadinessTimeout)
	}
	if cfg.MigrationDatabaseURL != cfg.DatabaseURL {
		t.Fatal("migration URL should fall back to runtime URL")
	}
	if cfg.SessionTTL != 12*time.Hour {
		t.Fatalf("SessionTTL = %s", cfg.SessionTTL)
	}
	if cfg.AttachmentStorageDir != ".local/attachments" || cfg.AttachmentMaxBytes != 100<<20 || cfg.AttachmentMaxCount != 10 {
		t.Fatalf("unexpected attachment defaults: %#v", cfg)
	}
}

func TestLoadWithLookupRejectsUnsafeAddress(t *testing.T) {
	t.Parallel()

	addresses := []string{"0.0.0.0:8080", ":8080", "192.168.1.5:8080", "[::]:8080", "127.0.0.1:80"}
	for _, address := range addresses {
		address := address
		t.Run(address, func(t *testing.T) {
			t.Parallel()
			_, err := LoadWithLookup(mapLookup(map[string]string{
				"DATABASE_URL": "postgres://example.invalid/ppr",
				"HTTP_ADDR":    address,
			}))
			if err == nil {
				t.Fatal("expected unsafe address to be rejected")
			}
		})
	}
}

func TestLoadWithLookupAcceptsExplicitValues(t *testing.T) {
	t.Parallel()

	cfg, err := LoadWithLookup(mapLookup(map[string]string{
		"APP_NAME":                        "Internal Register",
		"APP_ENV":                         "production",
		"HTTP_ADDR":                       "[::1]:9080",
		"BASE_PATH":                       "/backend",
		"DATABASE_URL":                    "postgres://runtime.invalid/ppr",
		"MIGRATION_DATABASE_URL":          "postgres://migration.invalid/ppr",
		"API_DOCS_ENABLED":                "true",
		"SESSION_CSRF_KEY":                "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		"SESSION_TTL":                     "24h",
		"BOOTSTRAP_TOKEN":                 "this-is-a-long-bootstrap-token",
		"READINESS_TIMEOUT":               "3s",
		"SHUTDOWN_TIMEOUT":                "15s",
		"ATTACHMENT_STORAGE_DIR":          "D:/private/ppr-attachments",
		"ATTACHMENT_MAX_FILE_BYTES":       "209715200",
		"ATTACHMENT_MAX_FILES_PER_UPDATE": "20",
	}))
	if err != nil {
		t.Fatalf("LoadWithLookup() error = %v", err)
	}
	if !cfg.APIDocsEnabled || cfg.Environment != "production" || cfg.BasePath != "/backend" || cfg.ReadinessTimeout != 3*time.Second || cfg.SessionTTL != 24*time.Hour || len(cfg.SessionCSRFKey) != 32 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if cfg.AttachmentMaxBytes != 200<<20 || cfg.AttachmentMaxCount != 20 {
		t.Fatalf("unexpected attachment config: %#v", cfg)
	}
}

func TestLoadWithLookupRejectsInvalidBasePath(t *testing.T) {
	t.Parallel()

	for _, basePath := range []string{"/", "backend", "/backend/", "/backend//api", "/backend/../api", "/backend api", "/backend?debug"} {
		basePath := basePath
		t.Run(basePath, func(t *testing.T) {
			t.Parallel()
			_, err := LoadWithLookup(mapLookup(map[string]string{
				"DATABASE_URL": "postgres://example.invalid/ppr",
				"BASE_PATH":    basePath,
			}))
			if err == nil {
				t.Fatal("expected invalid base path to be rejected")
			}
		})
	}
}

func TestLoadWithLookupRejectsInvalidIdentitySecrets(t *testing.T) {
	t.Parallel()

	tests := map[string]map[string]string{
		"short csrf key": {
			"DATABASE_URL":     "postgres://example.invalid/ppr",
			"SESSION_CSRF_KEY": "c2hvcnQ=",
		},
		"short bootstrap token": {
			"DATABASE_URL":    "postgres://example.invalid/ppr",
			"BOOTSTRAP_TOKEN": "too-short",
		},
	}
	for name, values := range tests {
		values := values
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := LoadWithLookup(mapLookup(values)); err == nil {
				t.Fatal("expected invalid identity configuration to fail")
			}
		})
	}
}

func TestLoadWithLookupRequiresDatabaseURL(t *testing.T) {
	t.Parallel()

	if _, err := LoadWithLookup(mapLookup(nil)); err == nil {
		t.Fatal("expected missing DATABASE_URL to fail")
	}
}

func mapLookup(values map[string]string) LookupEnv {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
