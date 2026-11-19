package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/adminanalysis"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/cluster"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/config"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/content"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/database"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/frontpage"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/ingest"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/jobs"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm/codex"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/media"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/newsletter"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/pipeline"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/politics"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/publish"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/reels"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/social"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/social/card"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/storyline"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/trends"
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
	codexClient := codex.NewClient(nil)
	gateway := llm.NewGateway(pool, codexClient)
	analysisStore := adminanalysis.NewStore(pool)
	model := gateway
	adminAnalysis := adminanalysis.NewService(analysisStore, model, codexClient)
	clusters := cluster.NewStore(pool)
	politicsStore := politics.NewStore(pool, model)
	pipelineStore := pipeline.NewStore(pool, model, clusters, politicsStore)
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
			TestSendReady: loaded.Newsletter.ResendAPIKey != "" && loaded.Newsletter.From != "",
		},
		time.Now,
	)
	mediaStore, err := media.New(processContext, media.Config{LocalDirectory: loaded.MediaLocalDirectory, R2AccessKeyID: loaded.R2AccessKeyID, R2AccountID: loaded.R2AccountID, R2Bucket: loaded.R2Bucket, R2SecretKey: loaded.R2SecretAccessKey})
	if err != nil {
		return fmt.Errorf("configure social media storage: %w", err)
	}
	chromeRenderer, renderErr := card.NewChromeRenderer(card.Options{ChromePath: loaded.Social.ChromePath})
	var socialRenderer card.Renderer
	if renderErr != nil {
		logger.Warn("social image renderer unavailable", "error", renderErr)
	} else {
		socialRenderer = chromeRenderer
	}
	socialService := social.NewService(social.NewStore(pool), loaded.Social, model, socialRenderer, mediaStore, time.Now)

	poller := ingest.NewPoller(pool, logger, clusters)
	frontPage := frontpage.NewService(frontpage.NewStore(pool), time.Now)
	frontPage.SetDesk(frontpage.Desk{Model: model, Illustrator: codexClient, Media: mediaStore, ImageModel: imageModel()})
	radar := trends.NewRadar(trends.Config{
		Store: trends.NewStore(pool), Storylines: storyline.NewStore(pool), Model: model,
		YouTube: trends.YouTube{Key: strings.TrimSpace(os.Getenv("SNAP_YOUTUBE_API_KEY"))}, Logger: logger,
	})
	if os.Getenv("SNAP_YOUTUBE_API_KEY") == "" {
		logger.Warn("trend radar runs without YouTube signals: SNAP_YOUTUBE_API_KEY is not set")
	}
	reelService := reels.NewService(reels.Config{
		Store: reels.NewStore(pool), Storylines: storyline.NewStore(pool), News: news, Model: model,
		Illustrator: codexClient, ImageModel: imageModel(), Media: mediaStore,
		Library:  reels.Library{Dir: reelMusicDirectory()},
		Renderer: reels.Renderer{ChromePath: loaded.Social.ChromePath, NoSandbox: chromeNoSandbox()},
		Logger:   logger, Radar: radar,
	})
	if err := reelService.Ready(); err != nil {
		logger.Warn("reel drafts unavailable on this worker", "error", err)
	}
	client, err := jobs.NewClient(pool, logger, poller, pipelineStore, contentStore, news, newsletterService, adminAnalysis, jobs.Extras{Social: socialService, FrontPage: frontPage, Storylines: storyline.NewService(storyline.NewStore(pool), model), Reels: reelService, Trends: radar})
	if err != nil {
		return err
	}
	// Auto-approved posts are uploaded promptly, and refreshed event coverage triggers an
	// early (debounced) social selection run instead of waiting for the 15-minute schedule.
	socialService.SetEnqueuers(func(ctx context.Context) error { return jobs.EnqueueSocialSelection(ctx, client) }, func(ctx context.Context) error { return jobs.EnqueueSocialPublisher(ctx, client) })
	pipelineStore.OnEventSynthesized(func(ctx context.Context, eventID string) {
		if err := jobs.EnqueueSocialRealtime(ctx, client); err != nil {
			logger.Warn("social realtime selection not enqueued", "event_id", eventID, "error", err)
		}
	})
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

// reelMusicDirectory holds the licensed Reel soundtracks (mounted read-only in production).
func reelMusicDirectory() string {
	if directory := strings.TrimSpace(os.Getenv("SNAP_REEL_MUSIC_DIR")); directory != "" {
		return directory
	}
	return ".data/reel-music"
}

// chromeNoSandbox matches the social card renderer: containers running Chrome need --no-sandbox.
func chromeNoSandbox() bool {
	enabled, _ := strconv.ParseBool(os.Getenv("SNAP_SOCIAL_CHROME_NO_SANDBOX"))
	return enabled || os.Getuid() == 0
}

// imageModel is the Codex model that drives the built-in image tool for illustrations.
func imageModel() string {
	if model := strings.TrimSpace(os.Getenv("SNAP_CODEX_IMAGE_MODEL")); model != "" {
		return model
	}
	if models := codex.Models(); len(models) > 0 {
		return models[0]
	}
	return ""
}
