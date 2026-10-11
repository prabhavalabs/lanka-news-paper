// Command edition builds one AI front-page edition and exits. Operators use it to rebuild the
// page by hand or to try the layout editor against a database without running the worker.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/config"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/database"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/frontpage"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm/codex"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/media"
)

func main() {
	force := flag.Bool("force", true, "build even overnight or when the stories are unchanged")
	images := flag.Bool("images", true, "draw illustrations")
	flag.Parse()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)
	if err := run(*force, *images); err != nil {
		logger.Error("edition build failed", "error", err)
		os.Exit(1)
	}
}

func run(force, images bool) error {
	config.LoadOptionalDotEnv(".env", "../../.env")
	loaded, err := config.FromEnvironment()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := database.Open(ctx, loaded.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	mediaStore, err := media.New(ctx, media.Config{
		LocalDirectory: loaded.MediaLocalDirectory, R2AccessKeyID: loaded.R2AccessKeyID, R2AccountID: loaded.R2AccountID,
		R2Bucket: loaded.R2Bucket, R2SecretKey: loaded.R2SecretAccessKey,
	})
	if err != nil {
		return err
	}
	codexClient := codex.NewClient(nil)
	desk := frontpage.Desk{Model: llm.NewGateway(pool, codexClient), Media: mediaStore}
	if images {
		desk.Illustrator, desk.ImageModel = codexClient, imageModel()
	}
	service := frontpage.NewService(frontpage.NewStore(pool), time.Now)
	service.SetDesk(desk)
	started := time.Now()
	if err := service.BuildEdition(ctx, force); err != nil {
		return err
	}
	slog.Info("edition built", "duration", time.Since(started).Round(time.Second))
	return nil
}

func imageModel() string {
	if model := strings.TrimSpace(os.Getenv("SNAP_CODEX_IMAGE_MODEL")); model != "" {
		return model
	}
	if models := codex.Models(); len(models) > 0 {
		return models[0]
	}
	return ""
}
