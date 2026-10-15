package jobs

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/storyline"
	"github.com/riverqueue/river"
	"time"
)

type StorylinesRefreshArgs struct{}

func (StorylinesRefreshArgs) Kind() string { return "storylines.refresh" }
func (StorylinesRefreshArgs) InsertOpts() river.InsertOpts {
	opts := socialInsertOpts()
	opts.Queue = "storylines"
	return opts
}

type StorylinesRefreshWorker struct {
	river.WorkerDefaults[StorylinesRefreshArgs]
	Storylines *storyline.Service
}

func (w *StorylinesRefreshWorker) Work(ctx context.Context, _ *river.Job[StorylinesRefreshArgs]) error {
	if w.Storylines == nil {
		return nil
	}
	return w.Storylines.Refresh(ctx, 48*time.Hour)
}
func (w *StorylinesRefreshWorker) Timeout(*river.Job[StorylinesRefreshArgs]) time.Duration {
	return 5 * time.Minute
}
func EnqueueStorylinesRefresh(ctx context.Context, client *river.Client[pgx.Tx]) error {
	_, err := client.Insert(ctx, StorylinesRefreshArgs{}, nil)
	return err
}
