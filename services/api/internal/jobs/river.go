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
	"github.com/riverqueue/river/rivertype"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/ingest"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/politics"
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

type NarrationArgs struct{}

func (NarrationArgs) Kind() string { return "intelligence.narration" }

func (NarrationArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue: "analysis",
		UniqueOpts: river.UniqueOpts{ByQueue: true, ByState: []rivertype.JobState{
			rivertype.JobStateAvailable,
			rivertype.JobStatePending,
			rivertype.JobStateRunning,
			rivertype.JobStateScheduled,
			rivertype.JobStateRetryable,
		}},
	}
}

type NarrationWorker struct {
	river.WorkerDefaults[NarrationArgs]
	Politics *politics.Store
}

func (worker *NarrationWorker) Work(ctx context.Context, _ *river.Job[NarrationArgs]) error {
	if worker.Politics == nil {
		return nil
	}
	return worker.Politics.Backfill(ctx, 10)
}

func (worker *NarrationWorker) Timeout(*river.Job[NarrationArgs]) time.Duration {
	return 8 * time.Minute
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

func NewClient(pool *pgxpool.Pool, logger *slog.Logger, poller *ingest.Poller, politicsStore *politics.Store, news *publish.Store) (*river.Client[pgx.Tx], error) {
	workers := river.NewWorkers()
	river.AddWorker(workers, &PollWorker{Poller: poller, News: news})
	river.AddWorker(workers, &NarrationWorker{Politics: politicsStore})
	river.AddWorker(workers, &BriefWorker{News: news})

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger: logger,
		PeriodicJobs: []*river.PeriodicJob{
			river.NewPeriodicJob(river.PeriodicInterval(time.Minute), func() (river.JobArgs, *river.InsertOpts) {
				return PollArgs{}, nil
			}, &river.PeriodicJobOpts{RunOnStart: true}),
			river.NewPeriodicJob(river.PeriodicInterval(time.Minute), func() (river.JobArgs, *river.InsertOpts) {
				return NarrationArgs{}, nil
			}, &river.PeriodicJobOpts{RunOnStart: true}),
			river.NewPeriodicJob(river.PeriodicInterval(time.Hour), func() (river.JobArgs, *river.InsertOpts) {
				return BriefArgs{}, nil
			}, nil),
		},
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 2},
			"analysis":         {MaxWorkers: 1},
		},
		Workers: workers,
	})
	if err != nil {
		return nil, fmt.Errorf("create river client: %w", err)
	}
	return client, nil
}
