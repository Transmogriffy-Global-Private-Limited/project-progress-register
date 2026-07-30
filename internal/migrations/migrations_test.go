package migrations

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestEmbeddedCameraVideoMigrationRetainsSourceAndVerificationConstraints(t *testing.T) {
	migrations, err := Load(embeddedFiles, "sql")
	if err != nil {
		t.Fatal(err)
	}
	latest := migrations[len(migrations)-1]
	if latest.Version != 9 || latest.Name != "camera_video_capture" {
		t.Fatalf("latest migration=%#v", latest)
	}
	for _, expected := range []string{
		"media_kind IN ('image', 'video')",
		"source <> 'camera'",
		"verification_status <> 'verified'",
		"source = 'camera'",
	} {
		if !strings.Contains(latest.SQL, expected) {
			t.Fatalf("migration 000009 missing %q", expected)
		}
	}
}

func TestLoadOrdersAndChecksumsMigrations(t *testing.T) {
	t.Parallel()

	migrations, err := Load(fstest.MapFS{
		"sql/000002_second.sql": {Data: []byte("SELECT 2;\n")},
		"sql/000001_first.sql":  {Data: []byte("SELECT 1;\n")},
		"sql/README.md":         {Data: []byte("ignored")},
	}, "sql")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(migrations) != 2 || migrations[0].Version != 1 || migrations[1].Version != 2 {
		t.Fatalf("unexpected migration order: %#v", migrations)
	}
	if migrations[0].Checksum == "" || migrations[0].Checksum == migrations[1].Checksum {
		t.Fatal("expected distinct non-empty checksums")
	}
}

func TestLoadRejectsInvalidMigration(t *testing.T) {
	t.Parallel()

	tests := map[string]fstest.MapFS{
		"bad filename": {
			"sql/1_bad.sql": {Data: []byte("SELECT 1;")},
		},
		"empty migration": {
			"sql/000001_empty.sql": {Data: []byte(" \n")},
		},
	}
	for name, files := range tests {
		files := files
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := Load(files, "sql"); err == nil {
				t.Fatal("expected invalid migration to fail")
			}
		})
	}
}
