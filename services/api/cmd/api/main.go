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

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/cluster"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/config"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/database"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/desk"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/httpapi"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/iam"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/ingest"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/jobs"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/media"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/pipeline"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/politics"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/publish"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/registry"
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
	politicsStore := politics.NewStore(pool, gateway)
	pipelineStore := pipeline.NewStore(pool, gateway, clusters, politicsStore)
	producer, err := jobs.NewProducer(pool, logger)
	if err != nil {
		return err
	}
	poller := ingest.NewPoller(pool, logger, clusters)
	startPipeline := func(ctx context.Context, articleID string) error {
		runID, err := pipelineStore.Start(ctx, articleID, "ingestion")
		if err != nil {
			return err
		}
		return jobs.EnqueuePipeline(ctx, producer, runID)
	}
	poller.SetArticlePipeline(startPipeline)
	news := publish.NewStore(pool)
	deskStore := desk.NewStore(pool)
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
			AllowedOrigins: loaded.AllowedOrigins,
			CookieSecure:   loaded.SessionCookieSecure,
			Database:       pool,
			Desk:           deskStore,
			IAM:            users,
			LLM:            gateway,
			Media:          mediaStore,
			News:           news,
			Poller:         poller,
			Registry:       registry.NewStore(pool),
			RetryPipeline: func(ctx context.Context, articleID, step string) error {
				runID, err := pipelineStore.Retry(ctx, articleID, step)
				if err != nil {
					return err
				}
				return jobs.EnqueuePipeline(ctx, producer, runID)
			},
			SessionTTL: loaded.SessionTTL,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      45 * time.Second,
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
