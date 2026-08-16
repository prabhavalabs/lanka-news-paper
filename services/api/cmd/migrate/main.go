package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/config"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/database"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/jobs"
)

func main() {
	if err := run(); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	config.LoadOptionalDotEnv(".env", "../../.env")
	loaded, err := config.FromEnvironment()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	absolutePath, err := filepath.Abs(loaded.MigrationsPath)
	if err != nil {
		return fmt.Errorf("resolve migration path: %w", err)
	}
	sourceURL := (&url.URL{Scheme: "file", Path: absolutePath}).String()

	runner, err := migrate.New(sourceURL, loaded.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create migration runner: %w", err)
	}
	defer func() {
		sourceError, databaseError := runner.Close()
		if sourceError != nil {
			slog.Error("close migration source", "error", sourceError)
		}
		if databaseError != nil {
			slog.Error("close migration database", "error", databaseError)
		}
	}()

	if err := runner.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	ctx := context.Background()
	pool, err := database.Open(ctx, loaded.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := jobs.Migrate(ctx, pool); err != nil {
		return err
	}

	version, dirty, err := runner.Version()
	if err != nil {
		return fmt.Errorf("read migration version: %w", err)
	}
	slog.Info("database migration complete", "version", version, "dirty", dirty)
	return nil
}
