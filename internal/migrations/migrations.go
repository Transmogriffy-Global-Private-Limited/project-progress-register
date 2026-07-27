package migrations

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const advisoryLockID int64 = 710274601

var migrationName = regexp.MustCompile(`^(\d{6})_([a-z0-9_]+)\.sql$`)

//go:embed sql
var embeddedFiles embed.FS

// Migration is one immutable, checksummed, forward-only database change.
type Migration struct {
	Version  int64
	Name     string
	SQL      string
	Checksum string
}

// Status describes the relationship between the database ledger and embedded migrations.
type Status struct {
	Initialized bool
	Applied     int
	Pending     int
}

// Runner applies and verifies the embedded migration sequence.
type Runner struct {
	pool       *pgxpool.Pool
	migrations []Migration
}

// New creates a migration runner from the embedded canonical SQL directory.
func New(pool *pgxpool.Pool) (*Runner, error) {
	if pool == nil {
		return nil, fmt.Errorf("PostgreSQL pool is required")
	}
	migrations, err := Load(embeddedFiles, "sql")
	if err != nil {
		return nil, err
	}
	return &Runner{pool: pool, migrations: migrations}, nil
}

// Load parses migration files from an arbitrary filesystem for deterministic testing.
func Load(source fs.FS, directory string) ([]Migration, error) {
	entries, err := fs.ReadDir(source, directory)
	if err != nil {
		return nil, fmt.Errorf("read migration directory: %w", err)
	}

	result := make([]Migration, 0, len(entries))
	seen := make(map[int64]string)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		matches := migrationName.FindStringSubmatch(entry.Name())
		if matches == nil {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}
		version, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil || version <= 0 {
			return nil, fmt.Errorf("invalid migration version in %q", entry.Name())
		}
		if previous, exists := seen[version]; exists {
			return nil, fmt.Errorf("migration version %06d is duplicated by %q and %q", version, previous, entry.Name())
		}
		body, err := fs.ReadFile(source, filepath.ToSlash(filepath.Join(directory, entry.Name())))
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}
		if strings.TrimSpace(string(body)) == "" {
			return nil, fmt.Errorf("migration %q is empty", entry.Name())
		}
		digest := sha256.Sum256(body)
		result = append(result, Migration{
			Version:  version,
			Name:     matches[2],
			SQL:      string(body),
			Checksum: hex.EncodeToString(digest[:]),
		})
		seen[version] = entry.Name()
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Version < result[j].Version })
	return result, nil
}

// Up applies every pending migration in its own transaction while holding a
// PostgreSQL advisory lock to prevent concurrent migrators.
func (r *Runner) Up(ctx context.Context) (Status, error) {
	connection, err := r.pool.Acquire(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("acquire migration connection: %w", err)
	}
	defer connection.Release()

	if _, err := connection.Exec(ctx, `SELECT pg_advisory_lock($1)`, advisoryLockID); err != nil {
		return Status{}, fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		unlockContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = connection.Exec(unlockContext, `SELECT pg_advisory_unlock($1)`, advisoryLockID)
	}()

	if _, err := connection.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS public.ppr_schema_migrations (
			version bigint PRIMARY KEY CHECK (version > 0),
			name text NOT NULL,
			checksum char(64) NOT NULL,
			applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
		)`); err != nil {
		return Status{}, fmt.Errorf("initialize migration ledger: %w", err)
	}

	applied, err := readApplied(ctx, connection)
	if err != nil {
		return Status{}, err
	}
	if err := r.validateApplied(applied); err != nil {
		return Status{}, err
	}

	for _, migration := range r.migrations {
		if _, exists := applied[migration.Version]; exists {
			continue
		}
		if err := applyOne(ctx, connection, migration); err != nil {
			return Status{}, err
		}
		applied[migration.Version] = migration.Checksum
	}

	return Status{Initialized: true, Applied: len(applied), Pending: 0}, nil
}

// Status reports migration state without changing the database.
func (r *Runner) Status(ctx context.Context) (Status, error) {
	var initialized bool
	if err := r.pool.QueryRow(ctx, `SELECT to_regclass('public.ppr_schema_migrations') IS NOT NULL`).Scan(&initialized); err != nil {
		return Status{}, fmt.Errorf("inspect migration ledger: %w", err)
	}
	if !initialized {
		return Status{Pending: len(r.migrations)}, nil
	}
	applied, err := readApplied(ctx, r.pool)
	if err != nil {
		return Status{}, err
	}
	if err := r.validateApplied(applied); err != nil {
		return Status{}, err
	}
	return Status{
		Initialized: true,
		Applied:     len(applied),
		Pending:     len(r.migrations) - len(applied),
	}, nil
}

// CheckCurrent implements the migration readiness boundary.
func (r *Runner) CheckCurrent(ctx context.Context) error {
	status, err := r.Status(ctx)
	if err != nil {
		return err
	}
	if !status.Initialized {
		return errors.New("migration ledger is not initialized")
	}
	if status.Pending != 0 {
		return fmt.Errorf("%d migration(s) are pending", status.Pending)
	}
	return nil
}

type rowQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func readApplied(ctx context.Context, queryer rowQuerier) (map[int64]string, error) {
	rows, err := queryer.Query(ctx, `SELECT version, checksum FROM public.ppr_schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("read migration ledger: %w", err)
	}
	defer rows.Close()

	applied := make(map[int64]string)
	for rows.Next() {
		var version int64
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, fmt.Errorf("scan migration ledger: %w", err)
		}
		applied[version] = strings.TrimSpace(checksum)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate migration ledger: %w", err)
	}
	return applied, nil
}

func (r *Runner) validateApplied(applied map[int64]string) error {
	known := make(map[int64]Migration, len(r.migrations))
	for _, migration := range r.migrations {
		known[migration.Version] = migration
	}
	for version, checksum := range applied {
		migration, exists := known[version]
		if !exists {
			return fmt.Errorf("database contains unknown migration version %06d", version)
		}
		if checksum != migration.Checksum {
			return fmt.Errorf("migration %06d checksum differs from the applied migration", version)
		}
	}
	return nil
}

func applyOne(ctx context.Context, connection *pgxpool.Conn, migration Migration) error {
	tx, err := connection.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration %06d: %w", migration.Version, err)
	}
	defer func() {
		rollbackContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = tx.Rollback(rollbackContext)
	}()

	if _, err := tx.Exec(ctx, migration.SQL); err != nil {
		return fmt.Errorf("execute migration %06d_%s: %w", migration.Version, migration.Name, err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO public.ppr_schema_migrations (version, name, checksum)
		VALUES ($1, $2, $3)`, migration.Version, migration.Name, migration.Checksum); err != nil {
		return fmt.Errorf("record migration %06d: %w", migration.Version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %06d: %w", migration.Version, err)
	}
	return nil
}
