package desk

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
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

// ArticleFilters narrows the admin article registry results.
type ArticleFilters struct {
	Status         string
	PipelineStatus string
	Category       string
	SourceID       string
	Since          *time.Time
}

func (store *Store) Articles(ctx context.Context, params pagination.Params, filters ArticleFilters) ([]ArticleListItem, int, error) {
	where := `
		FROM articles a
		JOIN sources s ON s.id = a.source_id
		LEFT JOIN categories c ON c.id = a.category_id
		LEFT JOIN LATERAL (
		  SELECT status, current_step, finished_at
		  FROM article_pipeline_runs WHERE article_id = a.id
		  ORDER BY created_at DESC LIMIT 1
		) run ON true
		WHERE ($1 = '' OR a.id::text = $1 OR a.headline ILIKE '%' || $1 || '%' OR s.name ILIKE '%' || $1 || '%')
		  AND (($2 = '' AND a.public_status <> 'removed') OR a.public_status = $2)
		  AND ($3 = '' OR COALESCE(run.status, 'not_started') = $3)
		  AND ($4 = '' OR c.slug = $4)
		  AND ($5 = '' OR a.source_id = NULLIF($5, '')::uuid)
		  AND ($6::timestamptz IS NULL OR a.received_at >= $6)`
	args := []any{
		params.Search,
		filters.Status,
		filters.PipelineStatus,
		filters.Category,
		filters.SourceID,
		filters.Since,
	}
	var total int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count articles: %w", err)
	}
	rows, err := store.pool.Query(ctx, `
		SELECT a.id::text, a.headline, a.public_status, s.name, COALESCE(s.icon_url, ''),
		       c.slug, a.received_at, a.published_at, run.status, run.current_step, run.finished_at
	`+where+`
		ORDER BY a.received_at DESC, a.id
		LIMIT $7 OFFSET $8
	`, append(args, params.Limit(), params.Offset())...)
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
	ProviderID  string         `json:"provider_id"`
	Model       string         `json:"model"`
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
	PipelineRunID  *string    `json:"pipeline_run_id"`
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

type ArticleContent struct {
	BodyText          string     `json:"body_text"`
	AcquisitionMethod string     `json:"acquisition_method"`
	SourceURL         string     `json:"source_url"`
	ExtractorVersion  string     `json:"extractor_version"`
	FetchedAt         time.Time  `json:"fetched_at"`
	RetentionUntil    *time.Time `json:"retention_until"`
	Characters        int        `json:"characters"`
}

type ArticleAnalysisDocument struct {
	OriginalText    string     `json:"original_text"`
	CleanedText     string     `json:"cleaned_text"`
	SummaryText     string     `json:"summary_text"`
	SummaryPoints   []string   `json:"summary_points"`
	CleanerVersion  string     `json:"cleaner_version"`
	CleanerProvider string     `json:"cleaner_provider"`
	CleanerModel    string     `json:"cleaner_model"`
	SummaryProvider string     `json:"summary_provider"`
	SummaryModel    string     `json:"summary_model"`
	CleanedAt       time.Time  `json:"cleaned_at"`
	SummarizedAt    *time.Time `json:"summarized_at"`
}

type EventSourceSpectrum struct {
	ArticleID         string  `json:"article_id"`
	SourceID          string  `json:"source_id"`
	Source            string  `json:"source"`
	SourceIcon        string  `json:"source_icon"`
	Headline          string  `json:"headline"`
	OriginalURL       string  `json:"original_url"`
	Label             string  `json:"label"`
	LeftProbability   float64 `json:"left_probability"`
	CenterProbability float64 `json:"center_probability"`
	RightProbability  float64 `json:"right_probability"`
	Confidence        float64 `json:"confidence"`
}

type EventNarrativeAnalysis struct {
	Summary          string                `json:"summary"`
	ArticleCount     int                   `json:"article_count"`
	SourceCount      int                   `json:"source_count"`
	RatedSourceCount int                   `json:"rated_source_count"`
	LeftPercentage   float64               `json:"left_percentage"`
	CenterPercentage float64               `json:"center_percentage"`
	RightPercentage  float64               `json:"right_percentage"`
	SourceSpectrum   []EventSourceSpectrum `json:"source_spectrum"`
	ProviderID       string                `json:"provider_id"`
	ProviderModel    string                `json:"provider_model"`
	AnalyzedAt       time.Time             `json:"analyzed_at"`
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
	Content             *ArticleContent           `json:"content"`
	AnalysisDocument    *ArticleAnalysisDocument  `json:"analysis_document"`
	EventAnalysis       *EventNarrativeAnalysis   `json:"event_analysis"`
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
	item.Content, err = store.articleContent(ctx, id)
	if err != nil {
		return ArticleDetail{}, err
	}
	item.AnalysisDocument, err = store.articleAnalysisDocument(ctx, id)
	if err != nil {
		return ArticleDetail{}, err
	}
	if eventID != nil {
		item.EventAnalysis, err = store.eventNarrativeAnalysis(ctx, *eventID)
		if err != nil {
			return ArticleDetail{}, err
		}
	}
	item.PipelineRuns, err = store.pipelineRuns(ctx, id)
	if err != nil {
		return ArticleDetail{}, err
	}
	item.LLMCalls, err = store.articleLLMCalls(ctx, id)
	return item, err
}

func (store *Store) articleContent(ctx context.Context, articleID string) (*ArticleContent, error) {
	var item ArticleContent
	err := store.pool.QueryRow(ctx, `
		SELECT body_text, acquisition_method, source_url, extractor_version,
		       fetched_at, retention_until, length(body_text)
		FROM article_contents
		WHERE article_id = $1 AND current
	`, articleID).Scan(
		&item.BodyText, &item.AcquisitionMethod, &item.SourceURL,
		&item.ExtractorVersion, &item.FetchedAt, &item.RetentionUntil,
		&item.Characters,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (store *Store) articleAnalysisDocument(ctx context.Context, articleID string) (*ArticleAnalysisDocument, error) {
	var item ArticleAnalysisDocument
	var points []byte
	err := store.pool.QueryRow(ctx, `
		SELECT original_text, cleaned_text, summary_text, summary_points,
		       cleaner_version, cleaner_provider, cleaner_model,
		       summary_provider, summary_model, cleaned_at, summarized_at
		FROM article_analysis_documents WHERE article_id = $1
	`, articleID).Scan(
		&item.OriginalText, &item.CleanedText, &item.SummaryText, &points,
		&item.CleanerVersion, &item.CleanerProvider, &item.CleanerModel,
		&item.SummaryProvider, &item.SummaryModel,
		&item.CleanedAt, &item.SummarizedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(points, &item.SummaryPoints); err != nil {
		return nil, err
	}
	if item.SummaryPoints == nil {
		item.SummaryPoints = []string{}
	}
	return &item, nil
}

func (store *Store) eventNarrativeAnalysis(ctx context.Context, eventID string) (*EventNarrativeAnalysis, error) {
	var item EventNarrativeAnalysis
	var spectrum []byte
	err := store.pool.QueryRow(ctx, `
		SELECT summary, article_count, source_count, rated_source_count,
		       left_percentage, center_percentage, right_percentage,
		       source_spectrum, provider_id, provider_model, analyzed_at
		FROM event_narrative_analyses WHERE event_id = $1
	`, eventID).Scan(
		&item.Summary, &item.ArticleCount, &item.SourceCount, &item.RatedSourceCount,
		&item.LeftPercentage, &item.CenterPercentage, &item.RightPercentage,
		&spectrum, &item.ProviderID, &item.ProviderModel, &item.AnalyzedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(spectrum, &item.SourceSpectrum); err != nil {
		return nil, err
	}
	if item.SourceSpectrum == nil {
		item.SourceSpectrum = []EventSourceSpectrum{}
	}
	rows, err := store.pool.Query(ctx, `
		SELECT id::text, headline, original_url
		FROM articles
		WHERE event_id = $1
	`, eventID)
	if err != nil {
		return nil, err
	}
	type articleLink struct{ headline, originalURL string }
	links := make(map[string]articleLink, len(item.SourceSpectrum))
	for rows.Next() {
		var id string
		var link articleLink
		if err := rows.Scan(&id, &link.headline, &link.originalURL); err != nil {
			rows.Close()
			return nil, err
		}
		links[id] = link
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for index := range item.SourceSpectrum {
		if link, ok := links[item.SourceSpectrum[index].ArticleID]; ok {
			item.SourceSpectrum[index].Headline = link.headline
			item.SourceSpectrum[index].OriginalURL = link.originalURL
		}
	}
	return &item, nil
}

func (store *Store) articlePolitical(ctx context.Context, articleID string) (*ArticlePoliticalAnalysis, error) {
	var item ArticlePoliticalAnalysis
	var mentions, evidence []byte
	err := store.pool.QueryRow(ctx, `
		SELECT model, economic_frame, confidence, mentions, relevant, label, rationale,
		       evidence, provider_id, provider_model, left_probability,
		       center_probability, right_probability, axis_version
		FROM article_political_analysis WHERE article_id = $1
	`, articleID).Scan(
		&item.Model, &item.EconomicFrame, &item.Confidence, &mentions, &item.Relevant,
		&item.Label, &item.Rationale, &evidence, &item.ProviderID, &item.ProviderModel,
		&item.LeftProbability, &item.CenterProbability, &item.RightProbability, &item.AxisVersion,
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
		SELECT id::text, status, trigger, COALESCE(provider_id, ''), COALESCE(provider_model, ''),
		       current_step, attempt, last_error,
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
		if err := rows.Scan(&run.ID, &run.Status, &run.Trigger, &run.ProviderID, &run.Model,
			&run.CurrentStep, &run.Attempt,
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
		SELECT id, pipeline_run_id::text, pipeline_step_id::text,
		       COALESCE(task, ''), COALESCE(provider_id, ''), COALESCE(model, ''),
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
		if err := rows.Scan(&item.ID, &item.PipelineRunID, &item.PipelineStepID,
			&item.Task, &item.ProviderID, &item.Model,
			&item.InputTokens, &item.OutputTokens, &item.LatencyMS, &item.FirstTokenMS, &item.Outcome,
			&item.Streamed, &item.ResponseText, &item.FinishReason, &item.ErrorDetail,
			&item.CreatedAt, &item.CompletedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ArticleLLMCalls returns a stable, paginated trace history for one article.
func (store *Store) ArticleLLMCalls(ctx context.Context, articleID string, params pagination.Params) ([]LLMCall, int, error) {
	var total int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM llm_calls WHERE article_id = $1`, articleID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count article LLM calls: %w", err)
	}
	rows, err := store.pool.Query(ctx, `
		SELECT id, pipeline_run_id::text, pipeline_step_id::text,
		       COALESCE(task, ''), COALESCE(provider_id, ''), COALESCE(model, ''),
		       input_tokens, output_tokens, latency_ms, first_token_ms, COALESCE(outcome, ''), streamed,
		       COALESCE(response_text, ''), COALESCE(finish_reason, ''), error_detail, created_at, completed_at
		FROM llm_calls
		WHERE article_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, articleID, params.Limit(), params.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list article LLM calls: %w", err)
	}
	defer rows.Close()
	items := make([]LLMCall, 0, params.Limit())
	for rows.Next() {
		var item LLMCall
		if err := rows.Scan(&item.ID, &item.PipelineRunID, &item.PipelineStepID,
			&item.Task, &item.ProviderID, &item.Model,
			&item.InputTokens, &item.OutputTokens, &item.LatencyMS, &item.FirstTokenMS, &item.Outcome,
			&item.Streamed, &item.ResponseText, &item.FinishReason, &item.ErrorDetail,
			&item.CreatedAt, &item.CompletedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

// DeleteArticleLLMCall permanently removes one model-call trace and records the administrator action.
func (store *Store) DeleteArticleLLMCall(ctx context.Context, articleID string, callID int64, actorID uuid.UUID) error {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var deletedID int64
	if err := tx.QueryRow(ctx, `
		DELETE FROM llm_calls
		WHERE id = $1 AND article_id = $2
		RETURNING id
	`, callID, articleID).Scan(&deletedID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_logs (actor_id, action, target_type, target_id, result)
		VALUES ($1, 'delete_llm_call', 'llm_call', $2, 'ok')
	`, actorID, fmt.Sprint(deletedID)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
