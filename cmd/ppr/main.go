package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/config"
	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/database"
	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/health"
	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/httpserver"
	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/identity"
	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/migrations"
	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/projects"
	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/safemarkdown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: ppr serve | ppr migrate up | ppr migrate status")
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	switch args[0] {
	case "serve":
		if len(args) != 1 {
			return errors.New("usage: ppr serve")
		}
		return serve(ctx, cfg, logger)
	case "migrate":
		if len(args) != 2 || (args[1] != "up" && args[1] != "status") {
			return errors.New("usage: ppr migrate up | ppr migrate status")
		}
		return migrate(ctx, cfg, args[1])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func serve(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	migrationRunner, err := migrations.New(pool)
	if err != nil {
		return fmt.Errorf("create migration runner: %w", err)
	}
	readiness, err := health.NewReadiness(pool, migrationRunner, cfg.ReadinessTimeout)
	if err != nil {
		return fmt.Errorf("create readiness checker: %w", err)
	}
	identityRepository, err := identity.NewPostgresRepository(pool)
	if err != nil {
		return fmt.Errorf("create identity repository: %w", err)
	}
	identityService, err := identity.NewService(ctx, identityRepository, identity.ServiceConfig{CSRFKey: cfg.SessionCSRFKey, SessionTTL: cfg.SessionTTL, BootstrapToken: cfg.BootstrapToken})
	if err != nil {
		return fmt.Errorf("create identity service: %w", err)
	}
	markdownRenderer := safemarkdown.New()
	projectRepository, err := projects.NewPostgresRepository(pool)
	if err != nil {
		return fmt.Errorf("create project repository: %w", err)
	}
	projectService, err := projects.NewService(projectRepository, markdownRenderer)
	if err != nil {
		return fmt.Errorf("create project service: %w", err)
	}
	handler, err := httpserver.New(httpserver.Options{
		AppName:        cfg.AppName,
		APIDocsEnabled: cfg.APIDocsEnabled,
		Logger:         logger,
		Readiness:      readiness,
		Identity:       identityService,
		Projects:       projectService,
		Production:     cfg.Environment == "production",
	})
	if err != nil {
		return fmt.Errorf("create HTTP handler: %w", err)
	}

	listener, err := net.Listen("tcp", cfg.HTTPAddress)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.HTTPAddress, err)
	}
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverError := make(chan error, 1)
	go func() {
		logger.Info("HTTP server listening", "address", listener.Addr().String(), "environment", cfg.Environment, "api_docs_enabled", cfg.APIDocsEnabled)
		serverError <- server.Serve(listener)
	}()

	select {
	case err := <-serverError:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("shut down HTTP server: %w", err)
		}
		if err := <-serverError; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("finish HTTP server: %w", err)
		}
		logger.Info("HTTP server stopped")
		return nil
	}
}

func migrate(ctx context.Context, cfg config.Config, command string) error {
	pool, err := database.NewPool(ctx, cfg.MigrationDatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	checkContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(checkContext); err != nil {
		return fmt.Errorf("connect to migration database: %w", err)
	}
	runner, err := migrations.New(pool)
	if err != nil {
		return fmt.Errorf("create migration runner: %w", err)
	}

	var status migrations.Status
	if command == "up" {
		status, err = runner.Up(ctx)
	} else {
		status, err = runner.Status(ctx)
	}
	if err != nil {
		return fmt.Errorf("migration %s: %w", command, err)
	}
	fmt.Printf("migration ledger initialized=%t applied=%d pending=%d\n", status.Initialized, status.Applied, status.Pending)
	return nil
}
