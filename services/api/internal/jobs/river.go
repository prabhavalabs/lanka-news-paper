package jobs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

type PollArgs struct{}

func (PollArgs) Kind() string { return "ingest.poll" }

type PollWorker struct {
	river.WorkerDefaults[PollArgs]
}

func (worker *PollWorker) Work(_ context.Context, _ *river.Job[PollArgs]) error {
	return nil
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

func NewClient(pool *pgxpool.Pool, logger *slog.Logger) (*river.Client[pgx.Tx], error) {
	workers := river.NewWorkers()
	river.AddWorker(workers, &PollWorker{})

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger: logger,
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
