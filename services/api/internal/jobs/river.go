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
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/pipeline"
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
}

func (worker *NarrationWorker) Work(_ context.Context, _ *river.Job[NarrationArgs]) error {
	return nil
}

func (worker *NarrationWorker) Timeout(*river.Job[NarrationArgs]) time.Duration {
	return 8 * time.Minute
}

type ArticlePipelineArgs struct {
	RunID string `json:"run_id"`
}

func (ArticlePipelineArgs) Kind() string { return "article.pipeline" }

func (args ArticlePipelineArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "analysis",
		MaxAttempts: 5,
		UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
			rivertype.JobStateAvailable,
			rivertype.JobStatePending,
			rivertype.JobStateRunning,
			rivertype.JobStateScheduled,
			rivertype.JobStateRetryable,
		}},
	}
}

type ArticlePipelineWorker struct {
	river.WorkerDefaults[ArticlePipelineArgs]
	Pipeline *pipeline.Store
}

func (worker *ArticlePipelineWorker) Work(ctx context.Context, job *river.Job[ArticlePipelineArgs]) error {
	return worker.Pipeline.Process(ctx, job.Args.RunID)
}

func (worker *ArticlePipelineWorker) Timeout(*river.Job[ArticlePipelineArgs]) time.Duration {
	return 20 * time.Minute
}

type PipelineDispatchArgs struct{}

func (PipelineDispatchArgs) Kind() string { return "article.pipeline.dispatch" }

func (PipelineDispatchArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByQueue: true, ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning,
		rivertype.JobStateScheduled, rivertype.JobStateRetryable,
	}}}
}

type PipelineDispatchWorker struct {
	river.WorkerDefaults[PipelineDispatchArgs]
	Pipeline *pipeline.Store
	Client   *river.Client[pgx.Tx]
}

func (worker *PipelineDispatchWorker) Work(ctx context.Context, _ *river.Job[PipelineDispatchArgs]) error {
	if err := worker.Pipeline.EnsureBacklog(ctx, 50); err != nil {
		return err
	}
	runIDs, err := worker.Pipeline.QueuedRuns(ctx, 100)
	if err != nil {
		return err
	}
	for _, runID := range runIDs {
		if err := EnqueuePipeline(ctx, worker.Client, runID); err != nil {
			return err
		}
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

func NewClient(pool *pgxpool.Pool, logger *slog.Logger, poller *ingest.Poller, pipelineStore *pipeline.Store, news *publish.Store) (*river.Client[pgx.Tx], error) {
	workers := river.NewWorkers()
	dispatcher := &PipelineDispatchWorker{Pipeline: pipelineStore}
	river.AddWorker(workers, &PollWorker{Poller: poller, News: news})
	river.AddWorker(workers, &NarrationWorker{})
	river.AddWorker(workers, &ArticlePipelineWorker{Pipeline: pipelineStore})
	river.AddWorker(workers, dispatcher)
	river.AddWorker(workers, &BriefWorker{News: news})

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger: logger,
		PeriodicJobs: []*river.PeriodicJob{
			river.NewPeriodicJob(river.PeriodicInterval(time.Minute), func() (river.JobArgs, *river.InsertOpts) {
				return PollArgs{}, nil
			}, &river.PeriodicJobOpts{RunOnStart: true}),
			river.NewPeriodicJob(river.PeriodicInterval(time.Minute), func() (river.JobArgs, *river.InsertOpts) {
				return PipelineDispatchArgs{}, nil
			}, &river.PeriodicJobOpts{RunOnStart: true}),
			river.NewPeriodicJob(river.PeriodicInterval(time.Hour), func() (river.JobArgs, *river.InsertOpts) {
				return BriefArgs{}, nil
			}, nil),
		},
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 2},
			"analysis":         {MaxWorkers: 5},
		},
		Workers: workers,
	})
	if err != nil {
		return nil, fmt.Errorf("create river client: %w", err)
	}
	dispatcher.Client = client
	return client, nil
}

func NewProducer(pool *pgxpool.Pool, logger *slog.Logger) (*river.Client[pgx.Tx], error) {
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Logger: logger})
	if err != nil {
		return nil, fmt.Errorf("create river producer: %w", err)
	}
	return client, nil
}

func EnqueuePipeline(ctx context.Context, client *river.Client[pgx.Tx], runID string) error {
	_, err := client.Insert(ctx, ArticlePipelineArgs{RunID: runID}, nil)
	return err
}
