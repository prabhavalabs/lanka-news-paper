package jobs

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/frontpage"
)

const editionQueue = "edition"

// EditionBuildArgs builds the next AI front-page edition. Force skips the overnight pause and
// the unchanged-stories check, for editor changes and manual rebuilds.
type EditionBuildArgs struct {
	Force bool `json:"force,omitempty"`
}

func (EditionBuildArgs) Kind() string { return "front_page.edition" }

func (EditionBuildArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: editionQueue, MaxAttempts: 2, UniqueOpts: river.UniqueOpts{ByQueue: true, ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateScheduled, rivertype.JobStateRetryable,
	}}}
}

type EditionBuildWorker struct {
	river.WorkerDefaults[EditionBuildArgs]
	FrontPage *frontpage.Service
}

func (worker *EditionBuildWorker) Work(ctx context.Context, job *river.Job[EditionBuildArgs]) error {
	if worker.FrontPage == nil {
		return nil
	}
	return worker.FrontPage.BuildEdition(ctx, job.Args.Force)
}

// Writing up to twenty reports and three illustrations through the Codex CLI takes a while.
func (worker *EditionBuildWorker) Timeout(*river.Job[EditionBuildArgs]) time.Duration {
	return 45 * time.Minute
}

// EnqueueEditionBuild queues a forced rebuild of the front page edition.
func EnqueueEditionBuild(ctx context.Context, client *river.Client[pgx.Tx]) error {
	_, err := client.Insert(ctx, EditionBuildArgs{Force: true}, nil)
	return err
}
