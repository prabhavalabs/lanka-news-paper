package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/riverqueue/river/rivertype"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/adminanalysis"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/content"
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

type ArticleContentArgs struct {
	ArticleID string `json:"article_id"`
}

func (ArticleContentArgs) Kind() string { return "article.content" }

func (args ArticleContentArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "crawl",
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

type ArticleContentWorker struct {
	river.WorkerDefaults[ArticleContentArgs]
	Content *content.Store
}

type AdminAnalysisBackfillDispatchArgs struct {
	RunID string `json:"run_id"`
}

func (AdminAnalysisBackfillDispatchArgs) Kind() string { return "admin.analysis.backfill.dispatch" }

func (args AdminAnalysisBackfillDispatchArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "admin-analysis",
		MaxAttempts: 5,
		UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
			rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning,
			rivertype.JobStateScheduled, rivertype.JobStateRetryable,
		}},
	}
}

type AdminArticleAnalysisArgs struct {
	RunID     string `json:"run_id"`
	ArticleID string `json:"article_id"`
}

func (AdminArticleAnalysisArgs) Kind() string { return "admin.article.analysis" }

func (args AdminArticleAnalysisArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "admin-analysis",
		Priority:    4,
		MaxAttempts: 5,
		UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
			rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning,
			rivertype.JobStateScheduled, rivertype.JobStateRetryable,
		}},
	}
}

type AdminAnalysisBackfillDispatchWorker struct {
	river.WorkerDefaults[AdminAnalysisBackfillDispatchArgs]
	Analysis *adminanalysis.Service
	Client   *river.Client[pgx.Tx]
}

type AdminArticleAnalysisWorker struct {
	river.WorkerDefaults[AdminArticleAnalysisArgs]
	Analysis *adminanalysis.Service
	Pipeline *pipeline.Store
}

const adminAnalysisDispatchBatchSize = 100

func (worker *AdminAnalysisBackfillDispatchWorker) Work(ctx context.Context, job *river.Job[AdminAnalysisBackfillDispatchArgs]) error {
	if err := worker.Analysis.Store().MarkRunStarted(ctx, job.Args.RunID); err != nil {
		if errors.Is(err, adminanalysis.ErrRunInactive) {
			return river.JobCancel(err)
		}
		return err
	}
	articleIDs, err := worker.Analysis.Store().PendingArticles(ctx, job.Args.RunID, adminAnalysisDispatchBatchSize)
	if err != nil {
		return err
	}
	for _, articleID := range articleIDs {
		result, err := worker.Client.Insert(ctx, AdminArticleAnalysisArgs{RunID: job.Args.RunID, ArticleID: articleID}, nil)
		if err != nil {
			return err
		}
		if err := worker.Analysis.Store().MarkQueued(ctx, job.Args.RunID, articleID, result.Job.ID); err != nil {
			if errors.Is(err, adminanalysis.ErrRunInactive) {
				_, _ = worker.Client.JobCancel(ctx, result.Job.ID)
				return river.JobCancel(err)
			}
			return err
		}
	}
	if len(articleIDs) == adminAnalysisDispatchBatchSize {
		return river.JobSnooze(time.Second)
	}
	return nil
}

func (worker *AdminArticleAnalysisWorker) Work(ctx context.Context, job *river.Job[AdminArticleAnalysisArgs]) error {
	if err := worker.Analysis.Store().MarkRunning(ctx, job.Args.RunID, job.Args.ArticleID, job.Attempt); err != nil {
		if errors.Is(err, adminanalysis.ErrRunInactive) {
			return river.JobCancel(err)
		}
		return err
	}
	run, err := worker.Analysis.Store().Get(ctx, job.Args.RunID)
	if err == nil && run.Workflow == "full_pipeline" {
		if worker.Pipeline == nil {
			err = errors.New("editorial pipeline is unavailable")
		}
		var pipelineRunID string
		if err == nil {
			pipelineRunID, err = worker.Pipeline.StartWithModel(
				ctx, job.Args.ArticleID, "admin_backfill", run.Provider, run.Model,
			)
		}
		if err == nil {
			err = worker.Pipeline.Process(ctx, pipelineRunID)
		}
		if err == nil {
			err = worker.Analysis.Store().MarkSucceeded(ctx, job.Args.RunID, job.Args.ArticleID)
		}
	} else if err == nil {
		err = worker.Analysis.Analyze(ctx, job.Args.RunID, job.Args.ArticleID, run.Provider, run.Model)
	}
	if err != nil {
		if errors.Is(err, adminanalysis.ErrRunInactive) {
			return river.JobCancel(err)
		}
		terminal := job.Attempt >= job.MaxAttempts
		if recordErr := worker.Analysis.Store().MarkAttemptFailed(ctx, job.Args.RunID, job.Args.ArticleID, err.Error(), terminal); recordErr != nil {
			if errors.Is(recordErr, adminanalysis.ErrRunInactive) {
				return river.JobCancel(err)
			}
			return errors.Join(err, recordErr)
		}
	}
	return err
}

func (worker *AdminArticleAnalysisWorker) Timeout(*river.Job[AdminArticleAnalysisArgs]) time.Duration {
	return 30 * time.Minute
}

type ArticleContentBackfillArgs struct{}

func (ArticleContentBackfillArgs) Kind() string { return "article.content.backfill" }

func (ArticleContentBackfillArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByQueue: true, ByState: []rivertype.JobState{
		rivertype.JobStateAvailable,
		rivertype.JobStatePending,
		rivertype.JobStateRunning,
		rivertype.JobStateScheduled,
		rivertype.JobStateRetryable,
	}}}
}

type ArticleContentBackfillWorker struct {
	river.WorkerDefaults[ArticleContentBackfillArgs]
	Content *content.Store
	Client  *river.Client[pgx.Tx]
}

const articleContentBackfillBatchSize = 25

func (worker *ArticleContentBackfillWorker) Work(ctx context.Context, _ *river.Job[ArticleContentBackfillArgs]) error {
	articleIDs, err := worker.Content.BackfillCandidates(ctx, articleContentBackfillBatchSize)
	if err != nil {
		return err
	}
	for _, articleID := range articleIDs {
		if err := enqueueBackfillContent(ctx, worker.Client, articleID); err != nil {
			return err
		}
	}
	if len(articleIDs) == articleContentBackfillBatchSize {
		return river.JobSnooze(30 * time.Second)
	}
	return nil
}

func (worker *ArticleContentWorker) Work(ctx context.Context, job *river.Job[ArticleContentArgs]) error {
	err := worker.Content.Enrich(ctx, job.Args.ArticleID)
	var rateLimit *content.RateLimitError
	if errors.As(err, &rateLimit) {
		return river.JobSnooze(rateLimit.RetryAfter)
	}
	return err
}

func (worker *ArticleContentWorker) Timeout(*river.Job[ArticleContentArgs]) time.Duration {
	return 2 * time.Minute
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

const queueHistoryRetention = 7 * 24 * time.Hour

type QueueHistoryCleanupArgs struct{}

func (QueueHistoryCleanupArgs) Kind() string { return "queue.history.cleanup" }

type QueueHistoryCleanupWorker struct {
	river.WorkerDefaults[QueueHistoryCleanupArgs]
	Pipeline *pipeline.Store
}

type ArticleContentCleanupArgs struct{}

func (ArticleContentCleanupArgs) Kind() string { return "article.content.cleanup" }

type ArticleContentCleanupWorker struct {
	river.WorkerDefaults[ArticleContentCleanupArgs]
	Content *content.Store
}

func (worker *ArticleContentCleanupWorker) Work(ctx context.Context, _ *river.Job[ArticleContentCleanupArgs]) error {
	for range 10 {
		deleted, err := worker.Content.DeleteExpired(ctx)
		if err != nil {
			return err
		}
		if deleted < 1000 {
			return nil
		}
	}
	return nil
}

func (worker *QueueHistoryCleanupWorker) Work(ctx context.Context, _ *river.Job[QueueHistoryCleanupArgs]) error {
	_, err := worker.Pipeline.DeleteHistory(ctx, queueHistoryCutoff(time.Now().UTC()))
	return err
}

func queueHistoryCutoff(now time.Time) time.Time {
	return now.Add(-queueHistoryRetention)
}

// PeriodicJobMetadata describes one scheduler-managed recurring job.
type PeriodicJobMetadata struct {
	Kind        string
	Name        string
	Description string
	Queue       string
	Interval    time.Duration
	RunOnStart  bool
}

type periodicJobDefinition struct {
	PeriodicJobMetadata
	Constructor func() (river.JobArgs, *river.InsertOpts)
}

func periodicJobDefinitions() []periodicJobDefinition {
	return []periodicJobDefinition{
		{
			PeriodicJobMetadata: PeriodicJobMetadata{
				Kind:        PollArgs{}.Kind(),
				Name:        "Source polling",
				Description: "Retrieves due publisher feeds and prepares new articles.",
				Queue:       river.QueueDefault,
				Interval:    time.Minute,
				RunOnStart:  true,
			},
			Constructor: func() (river.JobArgs, *river.InsertOpts) { return PollArgs{}, nil },
		},
		{
			PeriodicJobMetadata: PeriodicJobMetadata{
				Kind:        PipelineDispatchArgs{}.Kind(),
				Name:        "Pipeline dispatch",
				Description: "Moves queued article workflows onto analysis workers.",
				Queue:       river.QueueDefault,
				Interval:    time.Minute,
				RunOnStart:  true,
			},
			Constructor: func() (river.JobArgs, *river.InsertOpts) { return PipelineDispatchArgs{}, nil },
		},
		{
			PeriodicJobMetadata: PeriodicJobMetadata{
				Kind:        BriefArgs{}.Kind(),
				Name:        "News brief refresh",
				Description: "Regenerates the cached daily news brief.",
				Queue:       river.QueueDefault,
				Interval:    time.Hour,
			},
			Constructor: func() (river.JobArgs, *river.InsertOpts) { return BriefArgs{}, nil },
		},
		{
			PeriodicJobMetadata: PeriodicJobMetadata{
				Kind:        QueueHistoryCleanupArgs{}.Kind(),
				Name:        "Queue history cleanup",
				Description: "Deletes completed workflow telemetry after the retention window.",
				Queue:       river.QueueDefault,
				Interval:    24 * time.Hour,
				RunOnStart:  true,
			},
			Constructor: func() (river.JobArgs, *river.InsertOpts) { return QueueHistoryCleanupArgs{}, nil },
		},
		{
			PeriodicJobMetadata: PeriodicJobMetadata{
				Kind:        ArticleContentCleanupArgs{}.Kind(),
				Name:        "Article content retention",
				Description: "Deletes restricted full-text bodies when their rights retention window expires.",
				Queue:       river.QueueDefault,
				Interval:    24 * time.Hour,
				RunOnStart:  true,
			},
			Constructor: func() (river.JobArgs, *river.InsertOpts) { return ArticleContentCleanupArgs{}, nil },
		},
	}
}

// PeriodicJobCatalog returns a copy of the configured recurring-job metadata.
func PeriodicJobCatalog() []PeriodicJobMetadata {
	definitions := periodicJobDefinitions()
	catalog := make([]PeriodicJobMetadata, 0, len(definitions))
	for _, definition := range definitions {
		catalog = append(catalog, definition.PeriodicJobMetadata)
	}
	return catalog
}

func configuredPeriodicJobs() []*river.PeriodicJob {
	definitions := periodicJobDefinitions()
	configured := make([]*river.PeriodicJob, 0, len(definitions))
	for _, definition := range definitions {
		configured = append(configured, river.NewPeriodicJob(
			river.PeriodicInterval(definition.Interval),
			definition.Constructor,
			&river.PeriodicJobOpts{RunOnStart: definition.RunOnStart},
		))
	}
	return configured
}

// QueueMetadata describes one River queue and its configured concurrency.
type QueueMetadata struct {
	Name       string
	MaxWorkers int
}

// QueueCatalog returns a copy of the configured worker queues.
func QueueCatalog() []QueueMetadata {
	return []QueueMetadata{
		{Name: river.QueueDefault, MaxWorkers: 2},
		{Name: "analysis", MaxWorkers: 5},
		{Name: "crawl", MaxWorkers: 1},
		{Name: "admin-analysis", MaxWorkers: 1},
	}
}

func configuredQueues() map[string]river.QueueConfig {
	configured := make(map[string]river.QueueConfig)
	for _, queue := range QueueCatalog() {
		configured[queue.Name] = river.QueueConfig{MaxWorkers: queue.MaxWorkers}
	}
	return configured
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

func NewClient(pool *pgxpool.Pool, logger *slog.Logger, poller *ingest.Poller, pipelineStore *pipeline.Store, contentStore *content.Store, news *publish.Store, adminAnalysis *adminanalysis.Service) (*river.Client[pgx.Tx], error) {
	workers := river.NewWorkers()
	dispatcher := &PipelineDispatchWorker{Pipeline: pipelineStore}
	contentBackfill := &ArticleContentBackfillWorker{Content: contentStore}
	adminDispatcher := &AdminAnalysisBackfillDispatchWorker{Analysis: adminAnalysis}
	river.AddWorker(workers, &PollWorker{Poller: poller, News: news})
	river.AddWorker(workers, &NarrationWorker{})
	river.AddWorker(workers, &ArticlePipelineWorker{Pipeline: pipelineStore})
	river.AddWorker(workers, &ArticleContentWorker{Content: contentStore})
	river.AddWorker(workers, contentBackfill)
	river.AddWorker(workers, dispatcher)
	river.AddWorker(workers, &BriefWorker{News: news})
	river.AddWorker(workers, &QueueHistoryCleanupWorker{Pipeline: pipelineStore})
	river.AddWorker(workers, &ArticleContentCleanupWorker{Content: contentStore})
	if adminAnalysis != nil {
		river.AddWorker(workers, adminDispatcher)
		river.AddWorker(workers, &AdminArticleAnalysisWorker{Analysis: adminAnalysis, Pipeline: pipelineStore})
	}

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger:       logger,
		PeriodicJobs: configuredPeriodicJobs(),
		Queues:       configuredQueues(),
		Workers:      workers,
	})
	if err != nil {
		return nil, fmt.Errorf("create river client: %w", err)
	}
	dispatcher.Client = client
	contentBackfill.Client = client
	adminDispatcher.Client = client
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

func EnqueueContent(ctx context.Context, client *river.Client[pgx.Tx], articleID string) error {
	_, err := client.Insert(ctx, ArticleContentArgs{ArticleID: articleID}, nil)
	return err
}

func enqueueBackfillContent(ctx context.Context, client *river.Client[pgx.Tx], articleID string) error {
	_, err := client.Insert(ctx, ArticleContentArgs{ArticleID: articleID}, articleContentBackfillInsertOpts())
	return err
}

func articleContentBackfillInsertOpts() *river.InsertOpts {
	// River priority 1 is highest and 4 is lowest. Historical work must never
	// delay content retrieval for articles discovered by the live poller.
	return &river.InsertOpts{Priority: 4}
}

func EnqueueContentBackfill(ctx context.Context, client *river.Client[pgx.Tx]) error {
	_, err := client.Insert(ctx, ArticleContentBackfillArgs{}, nil)
	return err
}

func EnqueueAdminAnalysisBackfill(ctx context.Context, client *river.Client[pgx.Tx], runID string) error {
	_, err := client.Insert(ctx, AdminAnalysisBackfillDispatchArgs{RunID: runID}, nil)
	return err
}

func PauseAdminAnalysisBackfill(ctx context.Context, client *river.Client[pgx.Tx], store *adminanalysis.Store, runID string) (adminanalysis.Run, error) {
	result, err := store.Pause(ctx, runID)
	if err != nil {
		return adminanalysis.Run{}, err
	}
	if err := cancelAdminAnalysisJobs(ctx, client, result.JobIDs); err != nil {
		return result.Run, err
	}
	return result.Run, nil
}

func ResumeAdminAnalysisBackfill(ctx context.Context, client *river.Client[pgx.Tx], store *adminanalysis.Store, runID string) (adminanalysis.Run, error) {
	run, err := store.Resume(ctx, runID)
	if err != nil {
		return adminanalysis.Run{}, err
	}
	if err := EnqueueAdminAnalysisBackfill(ctx, client, runID); err != nil {
		_, _ = store.Pause(ctx, runID)
		return adminanalysis.Run{}, fmt.Errorf("enqueue resumed administrative backfill: %w", err)
	}
	return run, nil
}

func CancelAdminAnalysisBackfill(ctx context.Context, client *river.Client[pgx.Tx], store *adminanalysis.Store, runID string) (adminanalysis.Run, error) {
	result, err := store.Cancel(ctx, runID)
	if err != nil {
		return adminanalysis.Run{}, err
	}
	if err := cancelAdminAnalysisJobs(ctx, client, result.JobIDs); err != nil {
		return result.Run, err
	}
	return result.Run, nil
}

func cancelAdminAnalysisJobs(ctx context.Context, client *river.Client[pgx.Tx], jobIDs []int64) error {
	for _, jobID := range jobIDs {
		if _, err := client.JobCancel(ctx, jobID); err != nil && !errors.Is(err, river.ErrNotFound) {
			return fmt.Errorf("cancel administrative backfill job %d: %w", jobID, err)
		}
	}
	return nil
}
