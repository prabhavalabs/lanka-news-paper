package jobs

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/social"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// SocialSelectionArgs runs story selection. Trigger is "realtime" when new coverage arrived.
type SocialSelectionArgs struct {
	Trigger string `json:"trigger,omitempty"`
}

func (SocialSelectionArgs) Kind() string                 { return "social.selection" }
func (SocialSelectionArgs) InsertOpts() river.InsertOpts { return socialInsertOpts() }

type SocialPublishArgs struct{}

func (SocialPublishArgs) Kind() string                 { return "social.publish" }
func (SocialPublishArgs) InsertOpts() river.InsertOpts { return socialInsertOpts() }

func socialInsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: "social", MaxAttempts: 3, UniqueOpts: river.UniqueOpts{ByQueue: true, ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateScheduled, rivertype.JobStateRetryable,
	}}}
}

type SocialSelectionWorker struct {
	river.WorkerDefaults[SocialSelectionArgs]
	Social *social.Service
}

func (worker *SocialSelectionWorker) Work(ctx context.Context, job *river.Job[SocialSelectionArgs]) error {
	if worker.Social == nil {
		return nil
	}
	return worker.Social.Select(ctx, job.Args.Trigger)
}

// A run reviews and rewrites each draft, and settles posts left waiting, before it ends.
func (worker *SocialSelectionWorker) Timeout(*river.Job[SocialSelectionArgs]) time.Duration {
	return 30 * time.Minute
}

type SocialPublishWorker struct {
	river.WorkerDefaults[SocialPublishArgs]
	Social *social.Service
}

func (worker *SocialPublishWorker) Work(ctx context.Context, _ *river.Job[SocialPublishArgs]) error {
	if worker.Social == nil {
		return nil
	}
	return worker.Social.Publish(ctx)
}
func (worker *SocialPublishWorker) Timeout(*river.Job[SocialPublishArgs]) time.Duration {
	// Leaves room for the AI repeat check that runs just before an automatic post goes out.
	return 10 * time.Minute
}

func EnqueueSocialSelection(ctx context.Context, client *river.Client[pgx.Tx]) error {
	_, err := client.Insert(ctx, SocialSelectionArgs{}, nil)
	return err
}

// realtimeDelay collects a burst of new coverage into one selection run. A pending run
// (scheduled or realtime) absorbs further requests because selection jobs are unique per queue.
const realtimeDelay = 2 * time.Minute

func EnqueueSocialRealtime(ctx context.Context, client *river.Client[pgx.Tx]) error {
	opts := socialInsertOpts()
	opts.ScheduledAt = time.Now().Add(realtimeDelay)
	_, err := client.Insert(ctx, SocialSelectionArgs{Trigger: social.TriggerRealtime}, &opts)
	return err
}

func EnqueueSocialPublisher(ctx context.Context, client *river.Client[pgx.Tx]) error {
	_, err := client.Insert(ctx, SocialPublishArgs{}, nil)
	return err
}
