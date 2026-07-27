package health

import (
	"context"
	"fmt"
	"time"
)

// Database is the live PostgreSQL boundary used by readiness.
type Database interface {
	Ping(context.Context) error
}

// MigrationState verifies that the database schema matches the application.
type MigrationState interface {
	CheckCurrent(context.Context) error
}

// Readiness verifies dependencies required to serve stateful application traffic.
type Readiness struct {
	database   Database
	migrations MigrationState
	timeout    time.Duration
}

// NewReadiness constructs the database readiness boundary.
func NewReadiness(database Database, migrations MigrationState, timeout time.Duration) (*Readiness, error) {
	if database == nil || migrations == nil {
		return nil, fmt.Errorf("database and migration state are required")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("readiness timeout must be positive")
	}
	return &Readiness{database: database, migrations: migrations, timeout: timeout}, nil
}

// Check succeeds only when PostgreSQL is reachable and the schema is current.
func (r *Readiness) Check(ctx context.Context) error {
	checkContext, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	if err := r.database.Ping(checkContext); err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	if err := r.migrations.CheckCurrent(checkContext); err != nil {
		return fmt.Errorf("database schema not ready: %w", err)
	}
	return nil
}
