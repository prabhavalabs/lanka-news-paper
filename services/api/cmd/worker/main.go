package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/cluster"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/config"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/database"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/ingest"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/jobs"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/pipeline"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/politics"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/publish"
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

	if err := jobs.Migrate(processContext, pool); err != nil {
		return err
	}
	news := publish.NewStore(pool)
	model := llm.NewGateway(pool)
	clusters := cluster.NewStore(pool)
	politicsStore := politics.NewStore(pool, model)
	pipelineStore := pipeline.NewStore(pool, model, clusters, politicsStore)
	poller := ingest.NewPoller(pool, logger, clusters, model)
	client, err := jobs.NewClient(pool, logger, poller, politicsStore, pipelineStore, news)
	if err != nil {
		return err
	}
	poller.SetArticlePipeline(func(ctx context.Context, articleID string) error {
		runID, err := pipelineStore.Start(ctx, articleID, "ingestion")
		if err != nil {
			return err
		}
		return jobs.EnqueuePipeline(ctx, client, runID)
	})
	if err := client.Start(processContext); err != nil {
		return fmt.Errorf("start river: %w", err)
	}
	logger.Info("worker listening", "environment", loaded.Environment)

	<-processContext.Done()
	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), loaded.ShutdownTimeout)
	defer cancelShutdown()
	return client.Stop(shutdownContext)
}
