package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/config"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/content"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/database"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/jobs"
)

func main() {
	if err := run(); err != nil {
		slog.Error("content backfill dispatch failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	config.LoadOptionalDotEnv(".env", "../../.env")
	loaded, err := config.FromEnvironment()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	if err := loaded.BlockInSharedDevelopment("article content backfill"); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, loaded.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	store := content.NewStore(pool)
	report, err := store.BackfillReport(ctx)
	if err != nil {
		return err
	}
	producer, err := jobs.NewProducer(pool, slog.Default())
	if err != nil {
		return err
	}
	if err := jobs.EnqueueContentBackfill(ctx, producer); err != nil {
		return err
	}
	output := struct {
		content.BackfillReport
		DispatcherQueued bool `json:"dispatcher_queued"`
	}{BackfillReport: report, DispatcherQueued: true}
	encoded, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	return nil
}
