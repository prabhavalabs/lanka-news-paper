package desk

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/pagination"
)

type ArticleListItem struct {
	ID             string     `json:"id"`
	Headline       string     `json:"headline"`
	Status         string     `json:"public_status"`
	Source         string     `json:"source"`
	SourceIcon     string     `json:"source_icon"`
	Category       *string    `json:"category"`
	ReceivedAt     time.Time  `json:"received_at"`
	PublishedAt    time.Time  `json:"published_at"`
	PipelineStatus *string    `json:"pipeline_status"`
	CurrentStep    *string    `json:"current_step"`
	PipelineEnded  *time.Time `json:"pipeline_finished_at"`
}

func (store *Store) Articles(ctx context.Context, params pagination.Params, status, pipelineStatus string) ([]ArticleListItem, int, error) {
	where := `
		FROM articles a
		JOIN sources s ON s.id = a.source_id
		LEFT JOIN categories c ON c.id = a.category_id
		LEFT JOIN LATERAL (
		  SELECT status, current_step, finished_at
		  FROM article_pipeline_runs WHERE article_id = a.id
		  ORDER BY created_at DESC LIMIT 1
		) run ON true
		WHERE ($1 = '' OR a.headline ILIKE '%' || $1 || '%' OR s.name ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR a.public_status = $2)
		  AND ($3 = '' OR COALESCE(run.status, 'not_started') = $3)`
	var total int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) `+where, params.Search, status, pipelineStatus).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count articles: %w", err)
	}
	rows, err := store.pool.Query(ctx, `
		SELECT a.id::text, a.headline, a.public_status, s.name, COALESCE(s.icon_url, ''),
		       c.slug, a.received_at, a.published_at, run.status, run.current_step, run.finished_at
	`+where+`
		ORDER BY a.received_at DESC, a.id
		LIMIT $4 OFFSET $5
	`, params.Search, status, pipelineStatus, params.Limit(), params.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list articles: %w", err)
	}
	defer rows.Close()
	items := make([]ArticleListItem, 0, params.Limit())
	for rows.Next() {
		var item ArticleListItem
		if err := rows.Scan(
			&item.ID, &item.Headline, &item.Status, &item.Source, &item.SourceIcon,
			&item.Category, &item.ReceivedAt, &item.PublishedAt, &item.PipelineStatus,
			&item.CurrentStep, &item.PipelineEnded,
		); err != nil {
			return nil, 0, fmt.Errorf("scan article: %w", err)
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

type ArticleEvent struct {
	ID               string  `json:"id"`
	Title            string  `json:"title"`
	Confidence       float64 `json:"confidence"`
	AlgorithmVersion string  `json:"algorithm_version"`
}

type PipelineStep struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Position    int             `json:"position"`
	Status      string          `json:"status"`
	Attempt     int             `json:"attempt"`
	MaxAttempts int             `json:"max_attempts"`
	StartedAt   *time.Time      `json:"started_at"`
	FinishedAt  *time.Time      `json:"finished_at"`
	DurationMS  *int64          `json:"duration_ms"`
	ErrorDetail *string         `json:"error_detail"`
	Output      json.RawMessage `json:"output"`
	Logs        []PipelineLog   `json:"logs"`
}

type PipelineLog struct {
	ID        int64           `json:"id"`
	Level     string          `json:"level"`
	Event     string          `json:"event"`
	Message   string          `json:"message"`
	Details   json.RawMessage `json:"details"`
	CreatedAt time.Time       `json:"created_at"`
}

type PipelineRun struct {
	ID          string         `json:"id"`
	Status      string         `json:"status"`
	Trigger     string         `json:"trigger"`
	CurrentStep *string        `json:"current_step"`
	Attempt     int            `json:"attempt"`
	LastError   *string        `json:"last_error"`
	CreatedAt   time.Time      `json:"created_at"`
	StartedAt   *time.Time     `json:"started_at"`
	FinishedAt  *time.Time     `json:"finished_at"`
	Steps       []PipelineStep `json:"steps"`
}

type LLMCall struct {
	ID             int64      `json:"id"`
	PipelineStepID *string    `json:"pipeline_step_id"`
	Task           string     `json:"task"`
	ProviderID     string     `json:"provider_id"`
	Model          string     `json:"model"`
	InputTokens    *int       `json:"input_tokens"`
	OutputTokens   *int       `json:"output_tokens"`
	LatencyMS      *int       `json:"latency_ms"`
	FirstTokenMS   *int       `json:"first_token_ms"`
	Outcome        string     `json:"outcome"`
	Streamed       bool       `json:"streamed"`
	ResponseText   string     `json:"response_text"`
	FinishReason   string     `json:"finish_reason"`
	ErrorDetail    *string    `json:"error_detail"`
	CreatedAt      time.Time  `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at"`
}

type ArticleDetail struct {
	ID                  string                    `json:"id"`
	Headline            string                    `json:"headline"`
	Description         string                    `json:"description"`
	Status              string                    `json:"public_status"`
	SourceID            string                    `json:"source_id"`
	Source              string                    `json:"source"`
	SourceIcon          string                    `json:"source_icon"`
	OriginalURL         string                    `json:"original_url"`
	CanonicalURL        string                    `json:"canonical_url"`
	Author              string                    `json:"author"`
	Category            *string                   `json:"category"`
	CategoryName        *string                   `json:"category_name"`
	PublisherCategory   string                    `json:"publisher_category"`
	ClassificationModel *string                   `json:"classification_model"`
	ClassificationScore *float64                  `json:"classification_confidence"`
	EndpointID          string                    `json:"endpoint_id"`
	EndpointURL         string                    `json:"endpoint_url"`
	RightsMode          string                    `json:"rights_mode"`
	PublishedAt         time.Time                 `json:"published_at"`
	ReceivedAt          time.Time                 `json:"received_at"`
	Event               *ArticleEvent             `json:"event"`
	Political           *ArticlePoliticalAnalysis `json:"political"`
	PipelineRuns        []PipelineRun             `json:"pipeline_runs"`
	LLMCalls            []LLMCall                 `json:"llm_calls"`
}

func (store *Store) Article(ctx context.Context, id string) (ArticleDetail, error) {
	var item ArticleDetail
	var eventID, eventTitle, eventAlgorithm *string
	var eventConfidence *float64
	err := store.pool.QueryRow(ctx, `
		SELECT a.id::text, a.headline, COALESCE(a.description, ''), a.public_status,
		       s.id::text, s.name, COALESCE(s.icon_url, ''), a.original_url, a.canonical_url,
		       COALESCE(a.author, ''), c.slug, c.name_en, COALESCE(a.publisher_category, ''),
		       a.classify_model, a.classify_confidence, e.id::text, e.url, r.mode,
		       a.published_at, a.received_at, ec.id::text, ec.display_title,
		       ec.confidence, ec.algorithm_version
		FROM articles a
		JOIN sources s ON s.id = a.source_id
		JOIN source_endpoints e ON e.id = a.endpoint_id
		JOIN rights_profiles r ON r.id = a.rights_profile_id
		LEFT JOIN categories c ON c.id = a.category_id
		LEFT JOIN event_clusters ec ON ec.id = a.event_id
		WHERE a.id = $1
	`, id).Scan(
		&item.ID, &item.Headline, &item.Description, &item.Status,
		&item.SourceID, &item.Source, &item.SourceIcon, &item.OriginalURL, &item.CanonicalURL,
		&item.Author, &item.Category, &item.CategoryName, &item.PublisherCategory,
		&item.ClassificationModel, &item.ClassificationScore, &item.EndpointID, &item.EndpointURL,
		&item.RightsMode, &item.PublishedAt, &item.ReceivedAt, &eventID, &eventTitle,
		&eventConfidence, &eventAlgorithm,
	)
	if err != nil {
		return ArticleDetail{}, err
	}
	if eventID != nil {
		item.Event = &ArticleEvent{ID: *eventID, Title: *eventTitle, Confidence: *eventConfidence, AlgorithmVersion: *eventAlgorithm}
	}
	item.Political, err = store.articlePolitical(ctx, id)
	if err != nil {
		return ArticleDetail{}, err
	}
	item.PipelineRuns, err = store.pipelineRuns(ctx, id)
	if err != nil {
		return ArticleDetail{}, err
	}
	item.LLMCalls, err = store.articleLLMCalls(ctx, id)
	return item, err
}

func (store *Store) articlePolitical(ctx context.Context, articleID string) (*ArticlePoliticalAnalysis, error) {
	var item ArticlePoliticalAnalysis
	var mentions, evidence []byte
	err := store.pool.QueryRow(ctx, `
		SELECT model, economic_frame, confidence, mentions, relevant, label, rationale,
		       evidence, provider_id, provider_model
		FROM article_political_analysis WHERE article_id = $1
	`, articleID).Scan(
		&item.Model, &item.EconomicFrame, &item.Confidence, &mentions, &item.Relevant,
		&item.Label, &item.Rationale, &evidence, &item.ProviderID, &item.ProviderModel,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(mentions, &item.Mentions); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(evidence, &item.Evidence); err != nil {
		return nil, err
	}
	if item.Mentions == nil {
		item.Mentions = []PoliticalMention{}
	}
	if item.Evidence == nil {
		item.Evidence = []string{}
	}
	return &item, nil
}

func (store *Store) pipelineRuns(ctx context.Context, articleID string) ([]PipelineRun, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT id::text, status, trigger, current_step, attempt, last_error,
		       created_at, started_at, finished_at
		FROM article_pipeline_runs WHERE article_id = $1
		ORDER BY created_at DESC LIMIT 10
	`, articleID)
	if err != nil {
		return nil, err
	}
	var runs []PipelineRun
	for rows.Next() {
		var run PipelineRun
		if err := rows.Scan(&run.ID, &run.Status, &run.Trigger, &run.CurrentStep, &run.Attempt,
			&run.LastError, &run.CreatedAt, &run.StartedAt, &run.FinishedAt); err != nil {
			rows.Close()
			return nil, err
		}
		runs = append(runs, run)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for index := range runs {
		stepRows, err := store.pool.Query(ctx, `
			SELECT id::text, name, position, status, attempt, max_attempts, started_at,
			       finished_at, duration_ms, error_detail, output
			FROM article_pipeline_steps WHERE run_id = $1 ORDER BY position
		`, runs[index].ID)
		if err != nil {
			return nil, err
		}
		runs[index].Steps = make([]PipelineStep, 0, 3)
		for stepRows.Next() {
			var step PipelineStep
			if err := stepRows.Scan(&step.ID, &step.Name, &step.Position, &step.Status,
				&step.Attempt, &step.MaxAttempts, &step.StartedAt, &step.FinishedAt,
				&step.DurationMS, &step.ErrorDetail, &step.Output); err != nil {
				stepRows.Close()
				return nil, err
			}
			step.Logs = make([]PipelineLog, 0)
			runs[index].Steps = append(runs[index].Steps, step)
		}
		err = stepRows.Err()
		stepRows.Close()
		if err != nil {
			return nil, err
		}
		logRows, err := store.pool.Query(ctx, `
			SELECT step_id::text, id, level, event, message, details, created_at
			FROM article_pipeline_logs
			WHERE run_id = $1 AND step_id IS NOT NULL
			ORDER BY created_at, id
		`, runs[index].ID)
		if err != nil {
			return nil, err
		}
		stepIndexes := make(map[string]int, len(runs[index].Steps))
		for stepIndex, step := range runs[index].Steps {
			stepIndexes[step.ID] = stepIndex
		}
		for logRows.Next() {
			var stepID string
			var log PipelineLog
			if err := logRows.Scan(&stepID, &log.ID, &log.Level, &log.Event, &log.Message, &log.Details, &log.CreatedAt); err != nil {
				logRows.Close()
				return nil, err
			}
			if stepIndex, ok := stepIndexes[stepID]; ok {
				runs[index].Steps[stepIndex].Logs = append(runs[index].Steps[stepIndex].Logs, log)
			}
		}
		err = logRows.Err()
		logRows.Close()
		if err != nil {
			return nil, err
		}
	}
	return runs, nil
}

func (store *Store) articleLLMCalls(ctx context.Context, articleID string) ([]LLMCall, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT id, pipeline_step_id::text, COALESCE(task, ''), COALESCE(provider_id, ''), COALESCE(model, ''),
		       input_tokens, output_tokens, latency_ms, first_token_ms, COALESCE(outcome, ''), streamed,
		       COALESCE(response_text, ''), COALESCE(finish_reason, ''), error_detail, created_at, completed_at
		FROM llm_calls WHERE article_id = $1 ORDER BY created_at DESC LIMIT 50
	`, articleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]LLMCall, 0)
	for rows.Next() {
		var item LLMCall
		if err := rows.Scan(&item.ID, &item.PipelineStepID, &item.Task, &item.ProviderID, &item.Model,
			&item.InputTokens, &item.OutputTokens, &item.LatencyMS, &item.FirstTokenMS, &item.Outcome,
			&item.Streamed, &item.ResponseText, &item.FinishReason, &item.ErrorDetail,
			&item.CreatedAt, &item.CompletedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
