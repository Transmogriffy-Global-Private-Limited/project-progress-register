package health

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReadinessChecksDatabaseThenMigrations(t *testing.T) {
	t.Parallel()

	order := make([]string, 0, 2)
	checker, err := NewReadiness(
		fakeDatabase{check: func(context.Context) error { order = append(order, "database"); return nil }},
		fakeMigrations{check: func(context.Context) error { order = append(order, "migrations"); return nil }},
		time.Second,
	)
	if err != nil {
		t.Fatalf("NewReadiness() error = %v", err)
	}
	if err := checker.Check(context.Background()); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if len(order) != 2 || order[0] != "database" || order[1] != "migrations" {
		t.Fatalf("unexpected check order: %v", order)
	}
}

func TestReadinessStopsAfterDatabaseFailure(t *testing.T) {
	t.Parallel()

	migrationCalled := false
	checker, err := NewReadiness(
		fakeDatabase{check: func(context.Context) error { return errors.New("offline") }},
		fakeMigrations{check: func(context.Context) error { migrationCalled = true; return nil }},
		time.Second,
	)
	if err != nil {
		t.Fatalf("NewReadiness() error = %v", err)
	}
	if err := checker.Check(context.Background()); err == nil {
		t.Fatal("expected readiness failure")
	}
	if migrationCalled {
		t.Fatal("migration check should not run when database is unavailable")
	}
}

type fakeDatabase struct {
	check func(context.Context) error
}

func (f fakeDatabase) Ping(ctx context.Context) error { return f.check(ctx) }

type fakeMigrations struct {
	check func(context.Context) error
}

func (f fakeMigrations) CheckCurrent(ctx context.Context) error { return f.check(ctx) }
