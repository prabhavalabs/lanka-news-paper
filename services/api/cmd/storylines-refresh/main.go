// storylines-refresh groups running stories and refreshes their trend scores once.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/storyline"
)

func main() {
	window := flag.Duration("window", 48*time.Hour, "Event lookback (first run uses at least seven days)")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	url := os.Getenv("SNAP_DATABASE_URL")
	if url == "" {
		slog.Error("SNAP_DATABASE_URL is required")
		os.Exit(1)
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		slog.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err = storyline.NewService(storyline.NewStore(pool), llm.NewGateway(pool)).Refresh(ctx, *window); err != nil {
		slog.Error("storyline refresh failed", "error", err)
		os.Exit(1)
	}
}
