package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/config"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/database"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/jobs"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	config.LoadOptionalDotEnv(".env", "../../.env")
	loaded, err := config.FromEnvironment()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	processContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	connectContext, cancelConnect := context.WithTimeout(processContext, 10*time.Second)
	pool, err := database.Open(connectContext, loaded.DatabaseURL)
	cancelConnect()
	if err != nil {
		return err
	}
	defer pool.Close()

	client, err := jobs.NewClient(pool, logger)
	if err != nil {
		return err
	}
	if err := client.Start(processContext); err != nil {
		return fmt.Errorf("start river: %w", err)
	}
	logger.Info("worker listening", "environment", loaded.Environment)

	<-processContext.Done()
	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), loaded.ShutdownTimeout)
	defer cancelShutdown()
	return client.Stop(shutdownContext)
}
