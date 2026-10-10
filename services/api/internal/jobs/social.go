package jobs

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/social"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

type SocialSelectionArgs struct{}

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

func (worker *SocialSelectionWorker) Work(ctx context.Context, _ *river.Job[SocialSelectionArgs]) error {
	if worker.Social == nil {
		return nil
	}
	return worker.Social.Select(ctx)
}
func (worker *SocialSelectionWorker) Timeout(*river.Job[SocialSelectionArgs]) time.Duration {
	return 15 * time.Minute
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
	return 3 * time.Minute
}

func EnqueueSocialSelection(ctx context.Context, client *river.Client[pgx.Tx]) error {
	_, err := client.Insert(ctx, SocialSelectionArgs{}, nil)
	return err
}
func EnqueueSocialPublisher(ctx context.Context, client *river.Client[pgx.Tx]) error {
	_, err := client.Insert(ctx, SocialPublishArgs{}, nil)
	return err
}
