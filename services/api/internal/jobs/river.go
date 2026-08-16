package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/ingest"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/publish"
)

type PollArgs struct{}

func (PollArgs) Kind() string { return "ingest.poll" }

type PollWorker struct {
	river.WorkerDefaults[PollArgs]
	Poller *ingest.Poller
	News   *publish.Store
}

func (worker *PollWorker) Work(ctx context.Context, _ *river.Job[PollArgs]) error {
	if worker.Poller != nil {
		if err := worker.Poller.PollAll(ctx); err != nil {
			return err
		}
	}
	if worker.News != nil {
		return worker.News.WriteBrief(ctx)
	}
	return nil
}

type BriefArgs struct{}

func (BriefArgs) Kind() string { return "brief.daily" }

type BriefWorker struct {
	river.WorkerDefaults[BriefArgs]
	News *publish.Store
}

func (worker *BriefWorker) Work(ctx context.Context, _ *river.Job[BriefArgs]) error {
	if worker.News == nil {
		return nil
	}
	return worker.News.WriteBrief(ctx)
}

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("create river migrator: %w", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("apply river migrations: %w", err)
	}
	return nil
}

func NewClient(pool *pgxpool.Pool, logger *slog.Logger, poller *ingest.Poller, news *publish.Store) (*river.Client[pgx.Tx], error) {
	workers := river.NewWorkers()
	river.AddWorker(workers, &PollWorker{Poller: poller, News: news})
	river.AddWorker(workers, &BriefWorker{News: news})

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger: logger,
		PeriodicJobs: []*river.PeriodicJob{
			river.NewPeriodicJob(river.PeriodicInterval(time.Minute), func() (river.JobArgs, *river.InsertOpts) {
				return PollArgs{}, nil
			}, &river.PeriodicJobOpts{RunOnStart: true}),
			river.NewPeriodicJob(river.PeriodicInterval(time.Hour), func() (river.JobArgs, *river.InsertOpts) {
				return BriefArgs{}, nil
			}, nil),
		},
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 2},
		},
		Workers: workers,
	})
	if err != nil {
		return nil, fmt.Errorf("create river client: %w", err)
	}
	return client, nil
}
