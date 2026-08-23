package desk

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/pagination"
)

type QueueJob struct {
	ID          string         `json:"id"`
	JobID       *int64         `json:"job_id"`
	RunID       *string        `json:"run_id"`
	ArticleID   *string        `json:"article_id"`
	Title       string         `json:"title"`
	Source      *string        `json:"source"`
	SourceIcon  string         `json:"source_icon"`
	Kind        string         `json:"kind"`
	Queue       string         `json:"queue"`
	Status      string         `json:"status"`
	RiverState  string         `json:"river_state"`
	Trigger     *string        `json:"trigger"`
	Attempt     int            `json:"attempt"`
	MaxAttempts int            `json:"max_attempts"`
	CurrentStep *string        `json:"current_step"`
	CreatedAt   time.Time      `json:"created_at"`
	StartedAt   *time.Time     `json:"started_at"`
	FinishedAt  *time.Time     `json:"finished_at"`
	DurationMS  *int64         `json:"duration_ms"`
	ErrorDetail *string        `json:"error_detail"`
	ErrorTrace  *string        `json:"error_trace"`
	Steps       []PipelineStep `json:"steps"`
	runStatus   string
}

type QueueSummary struct {
	Total              int `json:"total"`
	Queued             int `json:"queued"`
	Processing         int `json:"processing"`
	Completed          int `json:"completed"`
	PartiallyCompleted int `json:"partially_completed"`
	Failed             int `json:"failed"`
}

type QueueMonitor struct {
	Items      []QueueJob      `json:"items"`
	Pagination pagination.Meta `json:"pagination"`
	Summary    QueueSummary    `json:"summary"`
}

const queueEntries = `
WITH pipeline_entries AS (
  SELECT
    'pipeline:' || run.id::text AS id,
    job.id AS job_id,
    run.id::text AS run_id,
    run.article_id::text AS article_id,
    article.headline AS title,
    source.name AS source,
    COALESCE(source.icon_url, '') AS source_icon,
    'article.pipeline'::text AS kind,
    COALESCE(job.queue, 'analysis') AS queue,
    CASE
      WHEN run.status = 'running' OR states.running > 0 THEN 'processing'
      WHEN states.failed > 0 AND states.succeeded > 0 THEN 'partially_completed'
      WHEN states.failed > 0 THEN 'failed'
      WHEN states.total > 0 AND states.terminal = states.total THEN 'completed'
      WHEN run.status = 'succeeded' THEN 'completed'
      WHEN run.status = 'failed' THEN 'failed'
      ELSE 'queued'
    END AS status,
    COALESCE(job.state, CASE WHEN run.status = 'running' THEN 'running' ELSE 'available' END) AS river_state,
    run.trigger,
    COALESCE(job.attempt, run.attempt)::integer AS attempt,
    COALESCE(job.max_attempts, 5)::integer AS max_attempts,
    run.current_step,
    run.created_at,
    COALESCE(job.attempted_at, run.started_at) AS started_at,
    COALESCE(job.finalized_at, run.finished_at) AS finished_at,
    CASE WHEN run.started_at IS NULL THEN NULL ELSE
      GREATEST(0, (extract(epoch FROM COALESCE(run.finished_at, clock_timestamp()) - run.started_at) * 1000)::bigint)
    END AS duration_ms,
    COALESCE(run.last_error, job.error_detail) AS error_detail,
    job.error_trace,
    run.status AS run_status
  FROM article_pipeline_runs run
  JOIN articles article ON article.id = run.article_id
  JOIN sources source ON source.id = article.source_id
  LEFT JOIN LATERAL (
    SELECT river.id, river.queue, river.state::text AS state, river.attempt, river.max_attempts,
           river.attempted_at, river.finalized_at,
           CASE WHEN cardinality(river.errors) > 0 THEN river.errors[cardinality(river.errors)]->>'error' END AS error_detail,
           CASE WHEN cardinality(river.errors) > 0 THEN river.errors[cardinality(river.errors)]->>'trace' END AS error_trace
    FROM river_job river
    WHERE river.kind = 'article.pipeline' AND river.args->>'run_id' = run.id::text
    ORDER BY river.created_at DESC, river.id DESC
    LIMIT 1
  ) job ON true
  CROSS JOIN LATERAL (
    SELECT count(*)::integer AS total,
           count(*) FILTER (WHERE step.status = 'running')::integer AS running,
           count(*) FILTER (WHERE step.status = 'failed')::integer AS failed,
           count(*) FILTER (WHERE step.status = 'succeeded')::integer AS succeeded,
           count(*) FILTER (WHERE step.status IN ('succeeded', 'skipped'))::integer AS terminal
    FROM article_pipeline_steps step WHERE step.run_id = run.id
  ) states
), generic_entries AS (
  SELECT
    'river:' || job.id::text AS id,
    job.id AS job_id,
    NULL::text AS run_id,
    NULL::text AS article_id,
    CASE job.kind
      WHEN 'ingest.poll' THEN 'Poll all source endpoints'
      WHEN 'article.pipeline.dispatch' THEN 'Dispatch queued article pipelines'
      WHEN 'article.content' THEN 'Retrieve approved full article content'
	  WHEN 'article.content.backfill' THEN 'Dispatch historical full article retrieval'
	  WHEN 'article.content.cleanup' THEN 'Delete expired full article content'
      WHEN 'brief.daily' THEN 'Generate daily news brief'
      WHEN 'intelligence.narration' THEN 'Run narration intelligence sweep'
      WHEN 'queue.history.cleanup' THEN 'Delete expired queue history'
      ELSE job.kind
    END AS title,
    NULL::text AS source,
    ''::text AS source_icon,
    job.kind,
    job.queue,
    CASE job.state::text
      WHEN 'running' THEN 'processing'
      WHEN 'completed' THEN 'completed'
      WHEN 'discarded' THEN 'failed'
      WHEN 'cancelled' THEN 'failed'
      ELSE 'queued'
    END AS status,
    job.state::text AS river_state,
    NULL::text AS trigger,
    job.attempt::integer,
    job.max_attempts::integer,
    NULL::text AS current_step,
    job.created_at,
    job.attempted_at AS started_at,
    job.finalized_at AS finished_at,
    CASE WHEN job.attempted_at IS NULL THEN NULL ELSE
      GREATEST(0, (extract(epoch FROM COALESCE(job.finalized_at, clock_timestamp()) - job.attempted_at) * 1000)::bigint)
    END AS duration_ms,
    CASE WHEN cardinality(job.errors) > 0 THEN job.errors[cardinality(job.errors)]->>'error' END AS error_detail,
    CASE WHEN cardinality(job.errors) > 0 THEN job.errors[cardinality(job.errors)]->>'trace' END AS error_trace,
    ''::text AS run_status
  FROM river_job job
  WHERE job.kind <> 'article.pipeline'
), entries AS (
  SELECT * FROM pipeline_entries
  UNION ALL
  SELECT * FROM generic_entries
)
`

const queueScope = `
WHERE ($1 = '' OR title ILIKE '%' || $1 || '%' OR COALESCE(source, '') ILIKE '%' || $1 || '%'
       OR id ILIKE '%' || $1 || '%' OR kind ILIKE '%' || $1 || '%')
  AND ($2 = '' OR queue = $2)
  AND ($3 = '' OR kind = $3)
  AND ($4::timestamptz IS NULL OR created_at >= $4)
`

func (store *Store) QueueJobs(ctx context.Context, params pagination.Params, status, queue, kind string, since *time.Time) (QueueMonitor, error) {
	var summary QueueSummary
	if err := store.pool.QueryRow(ctx, queueEntries+`
		SELECT count(*),
		       count(*) FILTER (WHERE status = 'queued'),
		       count(*) FILTER (WHERE status = 'processing'),
		       count(*) FILTER (WHERE status = 'completed'),
		       count(*) FILTER (WHERE status = 'partially_completed'),
		       count(*) FILTER (WHERE status = 'failed')
		FROM entries `+queueScope, params.Search, queue, kind, since).Scan(
		&summary.Total, &summary.Queued, &summary.Processing, &summary.Completed,
		&summary.PartiallyCompleted, &summary.Failed,
	); err != nil {
		return QueueMonitor{}, fmt.Errorf("summarize queue jobs: %w", err)
	}

	var total int
	if err := store.pool.QueryRow(ctx, queueEntries+`SELECT count(*) FROM entries `+queueScope+`
		AND ($5 = '' OR status = $5)`, params.Search, queue, kind, since, status).Scan(&total); err != nil {
		return QueueMonitor{}, fmt.Errorf("count queue jobs: %w", err)
	}

	rows, err := store.pool.Query(ctx, queueEntries+`
		SELECT id, job_id, run_id, article_id, title, source, source_icon, kind, queue,
		       status, river_state, trigger, attempt, max_attempts, current_step, created_at,
		       started_at, finished_at, duration_ms, error_detail, error_trace, run_status
		FROM entries `+queueScope+`
		  AND ($5 = '' OR status = $5)
		ORDER BY created_at DESC, id DESC
		LIMIT $6 OFFSET $7
	`, params.Search, queue, kind, since, status, params.Limit(), params.Offset())
	if err != nil {
		return QueueMonitor{}, fmt.Errorf("list queue jobs: %w", err)
	}
	defer rows.Close()

	items := make([]QueueJob, 0, params.Limit())
	runIDs := make([]string, 0, params.Limit())
	for rows.Next() {
		var item QueueJob
		if err := rows.Scan(
			&item.ID, &item.JobID, &item.RunID, &item.ArticleID, &item.Title, &item.Source,
			&item.SourceIcon, &item.Kind, &item.Queue, &item.Status, &item.RiverState,
			&item.Trigger, &item.Attempt, &item.MaxAttempts, &item.CurrentStep, &item.CreatedAt,
			&item.StartedAt, &item.FinishedAt, &item.DurationMS, &item.ErrorDetail,
			&item.ErrorTrace, &item.runStatus,
		); err != nil {
			return QueueMonitor{}, fmt.Errorf("scan queue job: %w", err)
		}
		item.Steps = make([]PipelineStep, 0)
		if item.RunID != nil {
			runIDs = append(runIDs, *item.RunID)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return QueueMonitor{}, fmt.Errorf("iterate queue jobs: %w", err)
	}

	steps, err := store.queueSteps(ctx, runIDs)
	if err != nil {
		return QueueMonitor{}, err
	}
	for index := range items {
		if items[index].RunID == nil {
			continue
		}
		items[index].Steps = steps[*items[index].RunID]
		items[index].Status = overallWorkflowStatus(items[index].runStatus, items[index].Steps)
	}

	return QueueMonitor{
		Items:      items,
		Pagination: pagination.NewMeta(params, total),
		Summary:    summary,
	}, nil
}

func (store *Store) queueSteps(ctx context.Context, runIDs []string) (map[string][]PipelineStep, error) {
	result := make(map[string][]PipelineStep, len(runIDs))
	if len(runIDs) == 0 {
		return result, nil
	}
	rows, err := store.pool.Query(ctx, `
		SELECT run_id::text, id::text, name, position, status, attempt, max_attempts,
		       started_at, finished_at, duration_ms, error_detail, output
		FROM article_pipeline_steps
		WHERE run_id = ANY($1::uuid[])
		ORDER BY run_id, position
	`, runIDs)
	if err != nil {
		return nil, fmt.Errorf("list queue job steps: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var runID string
		var item PipelineStep
		if err := rows.Scan(
			&runID, &item.ID, &item.Name, &item.Position, &item.Status, &item.Attempt,
			&item.MaxAttempts, &item.StartedAt, &item.FinishedAt, &item.DurationMS,
			&item.ErrorDetail, &item.Output,
		); err != nil {
			return nil, fmt.Errorf("scan queue job step: %w", err)
		}
		if item.Output == nil {
			item.Output = json.RawMessage(`{}`)
		}
		item.Logs = make([]PipelineLog, 0)
		result[runID] = append(result[runID], item)
	}
	return result, rows.Err()
}

func overallWorkflowStatus(runStatus string, steps []PipelineStep) string {
	succeeded, failed, terminal := 0, 0, 0
	for _, step := range steps {
		switch step.Status {
		case "running":
			return "processing"
		case "succeeded":
			succeeded++
			terminal++
		case "skipped":
			terminal++
		case "failed":
			failed++
		}
	}
	if runStatus == "running" {
		return "processing"
	}
	if failed > 0 && succeeded > 0 {
		return "partially_completed"
	}
	if failed > 0 {
		return "failed"
	}
	if len(steps) > 0 && terminal == len(steps) || runStatus == "succeeded" {
		return "completed"
	}
	if runStatus == "failed" {
		return "failed"
	}
	return "queued"
}
