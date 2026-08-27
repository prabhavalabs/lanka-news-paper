package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/adminanalysis"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/cluster"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/config"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/content"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/database"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/ingest"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/jobs"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/newsletter"
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
	if err := loaded.BlockInSharedDevelopment("worker"); err != nil {
		return err
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
	gateway := llm.NewGateway(pool)
	analysisStore := adminanalysis.NewStore(pool)
	codex := adminanalysis.NewCodexClient(nil)
	model := llm.NewRouter(gateway, adminanalysis.NewCodexProvider(analysisStore, codex))
	adminAnalysis := adminanalysis.NewService(analysisStore, model, codex)
	clusters := cluster.NewStore(pool)
	politicsStore := politics.NewStore(pool, model)
	pipelineStore := pipeline.NewStore(
		pool, model, clusters, politicsStore,
		pipeline.WithAdminProviderSort(os.Getenv("SNAP_ADMIN_ANALYSIS_PROVIDER_SORT")),
	)
	contentStore := content.NewStore(pool)
	newsletterStore := newsletter.NewStore(pool)
	if err := newsletterStore.SyncSettings(processContext, newsletter.Settings{
		Enabled: loaded.Newsletter.Enabled, Timezone: loaded.Newsletter.Timezone,
		SendHour: loaded.Newsletter.SendHour,
	}); err != nil {
		return err
	}
	if err := newsletterStore.ImportConfiguredRecipient(processContext, loaded.Newsletter.ConfiguredRecipient); err != nil {
		return err
	}
	newsletterService := newsletter.NewService(
		newsletterStore,
		newsletter.NewResendSender(loaded.Newsletter.ResendAPIKey, nil),
		model,
		newsletter.RuntimeConfig{
			BaseURL: loaded.Newsletter.BaseURL, From: loaded.Newsletter.From,
		},
		time.Now,
	)
	poller := ingest.NewPoller(pool, logger, clusters)
	client, err := jobs.NewClient(pool, logger, poller, pipelineStore, contentStore, news, newsletterService, adminAnalysis)
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
	poller.SetArticleContent(func(ctx context.Context, articleID, body, method string) (bool, error) {
		if _, err := contentStore.CaptureStructured(ctx, articleID, body, method); err != nil {
			return false, err
		}
		return contentStore.NeedsStaticFetch(ctx, articleID)
	}, func(ctx context.Context, articleID string) error {
		return jobs.EnqueueContent(ctx, client, articleID)
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
