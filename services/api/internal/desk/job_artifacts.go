package desk

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// QueueJobArtifact is a detail-only representation of data consumed or produced
// by a queue job. Data intentionally stays schemaless so new River job types can
// be inspected without expanding the queue-list response or its SSE snapshots.
type QueueJobArtifact struct {
	ID          string          `json:"id"`
	Role        string          `json:"role"`
	Kind        string          `json:"kind"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Data        json.RawMessage `json:"data"`
}

type QueueJobArtifacts struct {
	JobID   string             `json:"job_id"`
	Inputs  []QueueJobArtifact `json:"inputs"`
	Outputs []QueueJobArtifact `json:"outputs"`
}

type queueEntryRef struct {
	kind  string
	river int64
	run   uuid.UUID
}

type queueJobExecution struct {
	ID          int64
	Kind        string
	Queue       string
	State       string
	Args        json.RawMessage
	Periodic    bool
	Attempt     int
	MaxAttempts int
	CreatedAt   time.Time
	AttemptedAt *time.Time
	FinalizedAt *time.Time
	ScheduledAt time.Time
	ErrorDetail *string
}

func (store *Store) QueueJobArtifacts(ctx context.Context, id string) (QueueJobArtifacts, error) {
	reference, err := parseQueueEntryRef(id)
	if err != nil {
		return QueueJobArtifacts{}, pgx.ErrNoRows
	}

	result := QueueJobArtifacts{
		JobID:   id,
		Inputs:  make([]QueueJobArtifact, 0, 2),
		Outputs: make([]QueueJobArtifact, 0, 4),
	}
	if reference.kind == "pipeline" {
		return store.pipelineJobArtifacts(ctx, reference.run, result)
	}
	return store.riverJobArtifacts(ctx, reference.river, result)
}

func parseQueueEntryRef(id string) (queueEntryRef, error) {
	prefix, value, ok := strings.Cut(id, ":")
	if !ok || value == "" {
		return queueEntryRef{}, fmt.Errorf("invalid queue entry id")
	}
	switch prefix {
	case "river":
		jobID, err := strconv.ParseInt(value, 10, 64)
		if err != nil || jobID <= 0 {
			return queueEntryRef{}, fmt.Errorf("invalid river job id")
		}
		return queueEntryRef{kind: prefix, river: jobID}, nil
	case "pipeline":
		runID, err := uuid.Parse(value)
		if err != nil {
			return queueEntryRef{}, fmt.Errorf("invalid pipeline run id")
		}
		return queueEntryRef{kind: prefix, run: runID}, nil
	default:
		return queueEntryRef{}, fmt.Errorf("unsupported queue entry id")
	}
}

func (store *Store) riverJobArtifacts(ctx context.Context, jobID int64, result QueueJobArtifacts) (QueueJobArtifacts, error) {
	job, err := store.queueJobExecution(ctx, jobID)
	if err != nil {
		return QueueJobArtifacts{}, err
	}

	request, err := newQueueJobArtifact("input", "job_request", "River job request", "Arguments and scheduling metadata submitted to the worker.", map[string]any{
		"kind":         job.Kind,
		"queue":        job.Queue,
		"arguments":    job.Args,
		"periodic":     job.Periodic,
		"scheduled_at": job.ScheduledAt,
		"created_at":   job.CreatedAt,
	})
	if err != nil {
		return QueueJobArtifacts{}, err
	}
	result.Inputs = append(result.Inputs, request)

	articleID := jsonString(job.Args, "article_id")
	if articleID != "" {
		article, err := store.articleJobArtifact(ctx, articleID, "input")
		if err != nil && err != pgx.ErrNoRows {
			return QueueJobArtifacts{}, err
		}
		if err == nil {
			result.Inputs = append(result.Inputs, article)
		}
	}

	execution, err := executionArtifact(job)
	if err != nil {
		return QueueJobArtifacts{}, err
	}
	result.Outputs = append(result.Outputs, execution)

	switch job.Kind {
	case "article.content":
		if articleID != "" {
			if artifact, found, err := store.crawlAttemptArtifact(ctx, articleID, job); err != nil {
				return QueueJobArtifacts{}, err
			} else if found {
				result.Outputs = append(result.Outputs, artifact)
			}
			if artifact, found, err := store.articleContentArtifact(ctx, articleID, job); err != nil {
				return QueueJobArtifacts{}, err
			} else if found {
				result.Outputs = append(result.Outputs, artifact)
			}
		}
	case "ingest.poll":
		artifact, err := store.ingestionArtifact(ctx, job)
		if err != nil {
			return QueueJobArtifacts{}, err
		}
		result.Outputs = append(result.Outputs, artifact)
	case "article.pipeline.dispatch":
		artifact, err := store.dispatchArtifact(ctx, job, "article.pipeline")
		if err != nil {
			return QueueJobArtifacts{}, err
		}
		result.Outputs = append(result.Outputs, artifact)
	case "article.content.backfill":
		artifact, err := store.dispatchArtifact(ctx, job, "article.content")
		if err != nil {
			return QueueJobArtifacts{}, err
		}
		artifact.Kind = "backfill_summary"
		artifact.Title = "Full-article backfill dispatch"
		artifact.Description = "Article retrieval jobs created during this backfill execution."
		result.Outputs = append(result.Outputs, artifact)
	case "brief.daily":
		if artifact, found, err := store.dailyBriefArtifact(ctx, job); err != nil {
			return QueueJobArtifacts{}, err
		} else if found {
			result.Outputs = append(result.Outputs, artifact)
		}
	}

	return result, nil
}

func (store *Store) pipelineJobArtifacts(ctx context.Context, runID uuid.UUID, result QueueJobArtifacts) (QueueJobArtifacts, error) {
	var articleID, trigger, status string
	var createdAt time.Time
	var startedAt, finishedAt *time.Time
	err := store.pool.QueryRow(ctx, `
		SELECT article_id::text, trigger, status, created_at, started_at, finished_at
		FROM article_pipeline_runs
		WHERE id = $1
	`, runID).Scan(&articleID, &trigger, &status, &createdAt, &startedAt, &finishedAt)
	if err != nil {
		return QueueJobArtifacts{}, err
	}

	request, err := newQueueJobArtifact("input", "job_request", "Article workflow request", "The pipeline run and trigger submitted for analysis.", map[string]any{
		"run_id":     runID,
		"article_id": articleID,
		"trigger":    trigger,
		"created_at": createdAt,
	})
	if err != nil {
		return QueueJobArtifacts{}, err
	}
	result.Inputs = append(result.Inputs, request)
	article, err := store.articleJobArtifact(ctx, articleID, "input")
	if err != nil {
		return QueueJobArtifacts{}, err
	}
	result.Inputs = append(result.Inputs, article)

	duration := durationBetween(startedAt, finishedAt)
	execution, err := newQueueJobArtifact("output", "execution", "Workflow execution", "Terminal state and timing for this article workflow.", map[string]any{
		"state":       status,
		"started_at":  startedAt,
		"finished_at": finishedAt,
		"duration_ms": duration,
	})
	if err != nil {
		return QueueJobArtifacts{}, err
	}
	result.Outputs = append(result.Outputs, execution)

	steps, err := store.queueSteps(ctx, []string{runID.String()})
	if err != nil {
		return QueueJobArtifacts{}, err
	}
	for _, step := range steps[runID.String()] {
		artifact, err := newQueueJobArtifact("output", "pipeline_step", step.Name, "Persisted output from this pipeline step.", map[string]any{
			"name":         step.Name,
			"status":       step.Status,
			"attempt":      step.Attempt,
			"max_attempts": step.MaxAttempts,
			"started_at":   step.StartedAt,
			"finished_at":  step.FinishedAt,
			"duration_ms":  step.DurationMS,
			"error_detail": step.ErrorDetail,
			"output":       step.Output,
		})
		if err != nil {
			return QueueJobArtifacts{}, err
		}
		artifact.ID = "pipeline-step:" + step.ID
		result.Outputs = append(result.Outputs, artifact)
	}
	return result, nil
}

func (store *Store) queueJobExecution(ctx context.Context, jobID int64) (queueJobExecution, error) {
	var job queueJobExecution
	var args []byte
	err := store.pool.QueryRow(ctx, `
		SELECT id, kind, queue, state::text, args, COALESCE((metadata->>'periodic')::boolean, false),
		       attempt, max_attempts, created_at, attempted_at, finalized_at, scheduled_at,
		       CASE WHEN cardinality(errors) > 0 THEN errors[cardinality(errors)]->>'error' END
		FROM river_job
		WHERE id = $1
	`, jobID).Scan(
		&job.ID, &job.Kind, &job.Queue, &job.State, &args, &job.Periodic,
		&job.Attempt, &job.MaxAttempts, &job.CreatedAt, &job.AttemptedAt,
		&job.FinalizedAt, &job.ScheduledAt, &job.ErrorDetail,
	)
	job.Args = json.RawMessage(args)
	return job, err
}

func executionArtifact(job queueJobExecution) (QueueJobArtifact, error) {
	return newQueueJobArtifact("output", "execution", "Worker execution", "River state, attempts, timing, and terminal error for this job.", map[string]any{
		"state":        job.State,
		"attempt":      job.Attempt,
		"max_attempts": job.MaxAttempts,
		"started_at":   job.AttemptedAt,
		"finished_at":  job.FinalizedAt,
		"duration_ms":  durationBetween(job.AttemptedAt, job.FinalizedAt),
		"error_detail": job.ErrorDetail,
	})
}

func (store *Store) articleJobArtifact(ctx context.Context, articleID, role string) (QueueJobArtifact, error) {
	var data struct {
		ID          string    `json:"id"`
		Headline    string    `json:"headline"`
		Description string    `json:"description"`
		Author      string    `json:"author"`
		Source      string    `json:"source"`
		SourceIcon  string    `json:"source_icon"`
		OriginalURL string    `json:"original_url"`
		PublishedAt time.Time `json:"published_at"`
		ReceivedAt  time.Time `json:"received_at"`
		Status      string    `json:"status"`
		Category    *string   `json:"category"`
	}
	err := store.pool.QueryRow(ctx, `
		SELECT article.id::text, article.headline, COALESCE(article.description, ''),
		       COALESCE(article.author, ''), source.name, COALESCE(source.icon_url, ''),
		       article.original_url, article.published_at, article.received_at,
		       article.public_status, category.name_en
		FROM articles article
		JOIN sources source ON source.id = article.source_id
		LEFT JOIN categories category ON category.id = article.category_id
		WHERE article.id = $1
	`, articleID).Scan(
		&data.ID, &data.Headline, &data.Description, &data.Author, &data.Source,
		&data.SourceIcon, &data.OriginalURL, &data.PublishedAt, &data.ReceivedAt,
		&data.Status, &data.Category,
	)
	if err != nil {
		return QueueJobArtifact{}, err
	}
	return newQueueJobArtifact(role, "article", data.Headline, "Article record associated with this job.", data)
}

func (store *Store) crawlAttemptArtifact(ctx context.Context, articleID string, job queueJobExecution) (QueueJobArtifact, bool, error) {
	var data struct {
		Status              string     `json:"status"`
		RequestedURL        string     `json:"requested_url"`
		FinalURL            *string    `json:"final_url"`
		HTTPStatus          *int       `json:"http_status"`
		ResponseBytes       *int       `json:"response_bytes"`
		DurationMS          *int       `json:"duration_ms"`
		Extractor           *string    `json:"extractor"`
		ExtractedCharacters int        `json:"extracted_characters"`
		ErrorDetail         *string    `json:"error_detail"`
		StartedAt           time.Time  `json:"started_at"`
		FinishedAt          *time.Time `json:"finished_at"`
	}
	start, end := artifactWindow(job)
	err := store.pool.QueryRow(ctx, `
		SELECT status, requested_url, final_url, http_status, response_bytes, duration_ms,
		       extractor, extracted_characters, error_detail, started_at, finished_at
		FROM crawl_attempts
		WHERE article_id = $1 AND started_at BETWEEN $2 AND $3
		ORDER BY started_at DESC
		LIMIT 1
	`, articleID, start, end).Scan(
		&data.Status, &data.RequestedURL, &data.FinalURL, &data.HTTPStatus,
		&data.ResponseBytes, &data.DurationMS, &data.Extractor,
		&data.ExtractedCharacters, &data.ErrorDetail, &data.StartedAt, &data.FinishedAt,
	)
	if err == pgx.ErrNoRows {
		return QueueJobArtifact{}, false, nil
	}
	if err != nil {
		return QueueJobArtifact{}, false, err
	}
	artifact, err := newQueueJobArtifact("output", "crawl_attempt", "Retrieval attempt", "HTTP and extraction telemetry captured for this execution.", data)
	return artifact, true, err
}

func (store *Store) articleContentArtifact(ctx context.Context, articleID string, job queueJobExecution) (QueueJobArtifact, bool, error) {
	var data struct {
		BodyText          string     `json:"body_text"`
		AcquisitionMethod string     `json:"acquisition_method"`
		SourceURL         string     `json:"source_url"`
		ExtractorVersion  string     `json:"extractor_version"`
		FetchedAt         time.Time  `json:"fetched_at"`
		RetentionUntil    *time.Time `json:"retention_until"`
		Characters        int        `json:"characters"`
	}
	start, end := artifactWindow(job)
	err := store.pool.QueryRow(ctx, `
		SELECT body_text, acquisition_method, source_url, extractor_version,
		       fetched_at, retention_until, length(body_text)
		FROM article_contents
		WHERE article_id = $1 AND fetched_at BETWEEN $2 AND $3
		ORDER BY fetched_at DESC
		LIMIT 1
	`, articleID, start, end).Scan(
		&data.BodyText, &data.AcquisitionMethod, &data.SourceURL,
		&data.ExtractorVersion, &data.FetchedAt, &data.RetentionUntil,
		&data.Characters,
	)
	if err == pgx.ErrNoRows {
		return QueueJobArtifact{}, false, nil
	}
	if err != nil {
		return QueueJobArtifact{}, false, err
	}
	artifact, err := newQueueJobArtifact("output", "article_content", "Extracted full article", "Full text persisted by this retrieval job.", data)
	return artifact, true, err
}

func (store *Store) ingestionArtifact(ctx context.Context, job queueJobExecution) (QueueJobArtifact, error) {
	start, end := artifactWindow(job)
	rows, err := store.pool.Query(ctx, `
		SELECT run.id::text, source.name, endpoint.endpoint_type, endpoint.url,
		       run.status, run.http_status, run.item_count, run.new_item_count,
		       run.error_detail, run.started_at, run.ended_at
		FROM ingestion_runs run
		JOIN source_endpoints endpoint ON endpoint.id = run.endpoint_id
		JOIN sources source ON source.id = endpoint.source_id
		WHERE run.started_at BETWEEN $1 AND $2
		ORDER BY run.started_at, source.name
	`, start, end)
	if err != nil {
		return QueueJobArtifact{}, err
	}
	defer rows.Close()

	type ingestionRun struct {
		ID           string     `json:"id"`
		Source       string     `json:"source"`
		EndpointType string     `json:"endpoint_type"`
		EndpointURL  string     `json:"endpoint_url"`
		Status       string     `json:"status"`
		HTTPStatus   *int       `json:"http_status"`
		Items        int        `json:"items"`
		NewItems     int        `json:"new_items"`
		ErrorDetail  *string    `json:"error_detail"`
		StartedAt    time.Time  `json:"started_at"`
		FinishedAt   *time.Time `json:"finished_at"`
	}
	runs := make([]ingestionRun, 0, 24)
	totalItems, newItems, failed := 0, 0, 0
	for rows.Next() {
		var run ingestionRun
		if err := rows.Scan(
			&run.ID, &run.Source, &run.EndpointType, &run.EndpointURL, &run.Status,
			&run.HTTPStatus, &run.Items, &run.NewItems, &run.ErrorDetail,
			&run.StartedAt, &run.FinishedAt,
		); err != nil {
			return QueueJobArtifact{}, err
		}
		runs = append(runs, run)
		totalItems += run.Items
		newItems += run.NewItems
		if run.Status == "failed" {
			failed++
		}
	}
	if err := rows.Err(); err != nil {
		return QueueJobArtifact{}, err
	}
	return newQueueJobArtifact("output", "ingestion_summary", "Source polling results", "Endpoint-level ingestion runs associated with this polling execution.", map[string]any{
		"endpoints_checked": len(runs),
		"items_seen":        totalItems,
		"new_items":         newItems,
		"failed_endpoints":  failed,
		"runs":              runs,
	})
}

func (store *Store) dispatchArtifact(ctx context.Context, job queueJobExecution, dispatchedKind string) (QueueJobArtifact, error) {
	start, end := artifactWindow(job)
	// Backfill jobs can snooze and resume many times. Their output belongs to the
	// complete River job lifetime, not only the final attempt window.
	if job.Kind == "article.content.backfill" {
		start = job.CreatedAt.Add(-time.Second)
	}
	var count int
	var references []string
	err := store.pool.QueryRow(ctx, `
		SELECT count(*)::integer,
		       COALESCE(array_agg(COALESCE(args->>'article_id', args->>'run_id'))
		         FILTER (WHERE args ? 'article_id' OR args ? 'run_id'), ARRAY[]::text[])
		FROM river_job
		WHERE kind = $1 AND created_at BETWEEN $2 AND $3 AND id <> $4
	`, dispatchedKind, start, end, job.ID).Scan(&count, &references)
	if err != nil {
		return QueueJobArtifact{}, err
	}
	return newQueueJobArtifact("output", "dispatch_summary", "Pipeline dispatch", "Jobs created during this dispatcher execution.", map[string]any{
		"job_kind":    dispatchedKind,
		"jobs_queued": count,
		"references":  references,
	})
}

func (store *Store) dailyBriefArtifact(ctx context.Context, job queueJobExecution) (QueueJobArtifact, bool, error) {
	var data struct {
		Date      time.Time `json:"date"`
		Title     string    `json:"title"`
		Body      string    `json:"body"`
		Model     *string   `json:"model"`
		CreatedAt time.Time `json:"created_at"`
	}
	err := store.pool.QueryRow(ctx, `
		SELECT brief_date, title_si, body_si, model, created_at
		FROM daily_briefs
		WHERE brief_date = ($1 AT TIME ZONE 'Asia/Colombo')::date
	`, job.CreatedAt).Scan(&data.Date, &data.Title, &data.Body, &data.Model, &data.CreatedAt)
	if err == pgx.ErrNoRows {
		return QueueJobArtifact{}, false, nil
	}
	if err != nil {
		return QueueJobArtifact{}, false, err
	}
	artifact, err := newQueueJobArtifact("output", "daily_brief", data.Title, "Sinhala daily brief available after this job ran.", data)
	return artifact, true, err
}

func newQueueJobArtifact(role, kind, title, description string, data any) (QueueJobArtifact, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return QueueJobArtifact{}, fmt.Errorf("marshal %s artifact: %w", kind, err)
	}
	return QueueJobArtifact{
		ID:          role + ":" + kind,
		Role:        role,
		Kind:        kind,
		Title:       title,
		Description: description,
		Data:        payload,
	}, nil
}

func jsonString(data json.RawMessage, key string) string {
	var values map[string]json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		return ""
	}
	var value string
	_ = json.Unmarshal(values[key], &value)
	return value
}

func artifactWindow(job queueJobExecution) (time.Time, time.Time) {
	start := job.CreatedAt.Add(-time.Second)
	if job.AttemptedAt != nil {
		start = job.AttemptedAt.Add(-time.Second)
	}
	end := time.Now().UTC().Add(time.Second)
	if job.FinalizedAt != nil {
		end = job.FinalizedAt.Add(time.Second)
	}
	return start, end
}

func durationBetween(start, end *time.Time) *int64 {
	if start == nil {
		return nil
	}
	finished := time.Now().UTC()
	if end != nil {
		finished = *end
	}
	value := finished.Sub(*start).Milliseconds()
	if value < 0 {
		value = 0
	}
	return &value
}
