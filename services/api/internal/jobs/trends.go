package jobs

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/trends"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

const trendsQueue = "trends"

func trendsInsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: trendsQueue, MaxAttempts: 1, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateScheduled,
	}}}
}

// TrendsCollectArgs refreshes YouTube statistics for the watched Sri Lankan channels every hour,
// building the 30-day baseline the radar compares new videos with.
type TrendsCollectArgs struct{}

func (TrendsCollectArgs) Kind() string                 { return "trends.collect" }
func (TrendsCollectArgs) InsertOpts() river.InsertOpts { return trendsInsertOpts() }

type TrendsCollectWorker struct {
	river.WorkerDefaults[TrendsCollectArgs]
	Radar *trends.Radar
}

func (worker *TrendsCollectWorker) Work(ctx context.Context, _ *river.Job[TrendsCollectArgs]) error {
	if worker.Radar == nil {
		return nil
	}
	return worker.Radar.Collect(ctx)
}

func (worker *TrendsCollectWorker) Timeout(*river.Job[TrendsCollectArgs]) time.Duration {
	return 5 * time.Minute
}

// TrendsRadarArgs runs a full radar pass on demand (the reel builder also runs one when stale).
type TrendsRadarArgs struct{}

func (TrendsRadarArgs) Kind() string                 { return "trends.radar" }
func (TrendsRadarArgs) InsertOpts() river.InsertOpts { return trendsInsertOpts() }

type TrendsRadarWorker struct {
	river.WorkerDefaults[TrendsRadarArgs]
	Radar *trends.Radar
}

func (worker *TrendsRadarWorker) Work(ctx context.Context, _ *river.Job[TrendsRadarArgs]) error {
	if worker.Radar == nil {
		return nil
	}
	_, err := worker.Radar.Run(ctx)
	return err
}

func (worker *TrendsRadarWorker) Timeout(*river.Job[TrendsRadarArgs]) time.Duration {
	return 20 * time.Minute
}

func EnqueueTrendRadar(ctx context.Context, client *river.Client[pgx.Tx]) error {
	_, err := client.Insert(ctx, TrendsRadarArgs{}, nil)
	return err
}
