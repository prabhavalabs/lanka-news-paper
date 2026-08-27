package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/adminanalysis"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/cluster"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/config"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/content"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/database"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/desk"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/httpapi"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/iam"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/ingest"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/jobs"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/media"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/newsletter"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/pipeline"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/politics"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/publish"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/registry"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/watchtower"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("API stopped", "error", err)
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

	users := iam.NewStore(pool)
	if err := users.Bootstrap(processContext, loaded.BootstrapAdminEmail, loaded.BootstrapAdminPasswordHash); err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}

	clusters := cluster.NewStore(pool)
	gateway := llm.NewGateway(pool)
	analysisStore := adminanalysis.NewStore(pool)
	codex := adminanalysis.NewCodexClient(nil)
	model := llm.NewRouter(gateway, adminanalysis.NewCodexProvider(analysisStore, codex))
	adminAnalysis := adminanalysis.NewService(analysisStore, model, codex)
	politicsStore := politics.NewStore(pool, model)
	pipelineStore := pipeline.NewStore(pool, model, clusters, politicsStore)
	watchTower := watchtower.NewService(watchtower.NewStore(pool), model, time.Now)
	producer, err := jobs.NewProducer(pool, logger)
	if err != nil {
		return err
	}
	poller := ingest.NewPoller(pool, logger, clusters)
	contentStore := content.NewStore(pool)
	startPipeline := func(ctx context.Context, articleID string) error {
		runID, err := pipelineStore.Start(ctx, articleID, "ingestion")
		if err != nil {
			return err
		}
		return jobs.EnqueuePipeline(ctx, producer, runID)
	}
	poller.SetArticlePipeline(startPipeline)
	poller.SetArticleContent(func(ctx context.Context, articleID, body, method string) (bool, error) {
		if _, err := contentStore.CaptureStructured(ctx, articleID, body, method); err != nil {
			return false, err
		}
		return contentStore.NeedsStaticFetch(ctx, articleID)
	}, func(ctx context.Context, articleID string) error {
		return jobs.EnqueueContent(ctx, producer, articleID)
	})
	news := publish.NewStore(pool)
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
	deskStore := desk.NewStore(pool)
	monitorBroker := desk.NewMonitorBroker(processContext, pool, logger)
	mediaStore, err := media.New(processContext, media.Config{
		LocalDirectory: loaded.MediaLocalDirectory,
		R2AccessKeyID:  loaded.R2AccessKeyID,
		R2AccountID:    loaded.R2AccountID,
		R2Bucket:       loaded.R2Bucket,
		R2SecretKey:    loaded.R2SecretAccessKey,
	})
	if err != nil {
		return fmt.Errorf("configure media storage: %w", err)
	}
	logger.Info("media storage ready", "remote", mediaStore.Remote())

	server := &http.Server{
		Addr: loaded.Address,
		Handler: httpapi.NewRouter(httpapi.Dependencies{
			AdminAnalysis:  adminAnalysis,
			AllowedOrigins: loaded.AllowedOrigins,
			CookieSecure:   loaded.SessionCookieSecure,
			Database:       pool,
			Desk:           deskStore,
			IAM:            users,
			LLM:            gateway,
			Media:          mediaStore,
			Monitor:        monitorBroker,
			News:           news,
			Newsletter:     newsletterStore,
			Poller:         poller,
			Registry:       registry.NewStore(pool),
			RunContentBackfill: func(ctx context.Context) error {
				return jobs.EnqueueContentBackfill(ctx, producer)
			},
			RunAdminAnalysisBackfill: func(ctx context.Context, runID string) error {
				return jobs.EnqueueAdminAnalysisBackfill(ctx, producer, runID)
			},
			PauseAdminAnalysisBackfill: func(ctx context.Context, runID string) (adminanalysis.Run, error) {
				return jobs.PauseAdminAnalysisBackfill(ctx, producer, adminAnalysis.Store(), runID)
			},
			ResumeAdminAnalysisBackfill: func(ctx context.Context, runID string) (adminanalysis.Run, error) {
				return jobs.ResumeAdminAnalysisBackfill(ctx, producer, adminAnalysis.Store(), runID)
			},
			CancelAdminAnalysisBackfill: func(ctx context.Context, runID string) (adminanalysis.Run, error) {
				return jobs.CancelAdminAnalysisBackfill(ctx, producer, adminAnalysis.Store(), runID)
			},
			RunPipeline: func(ctx context.Context, articleID, step string) error {
				runID, err := pipelineStore.Run(ctx, articleID, step)
				if err != nil {
					return err
				}
				return jobs.EnqueuePipeline(ctx, producer, runID)
			},
			SessionTTL: loaded.SessionTTL,
			WatchTower: watchTower,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      75 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("API listening", "address", loaded.Address, "environment", loaded.Environment)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case serverError := <-serverErrors:
		if !errors.Is(serverError, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", serverError)
		}
		return nil
	case <-processContext.Done():
		shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), loaded.ShutdownTimeout)
		defer cancelShutdown()
		if err := server.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		return nil
	}
}
