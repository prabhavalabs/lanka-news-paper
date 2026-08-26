package adminanalysis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

type Preview struct {
	Articles int `json:"articles"`
}

type Run struct {
	ID                  string     `json:"id"`
	Scope               string     `json:"scope"`
	Workflow            string     `json:"workflow"`
	Provider            string     `json:"provider"`
	Model               string     `json:"model"`
	From                *time.Time `json:"from"`
	To                  *time.Time `json:"to"`
	ArticleID           *string    `json:"article_id"`
	Status              string     `json:"status"`
	TotalArticles       int        `json:"total_articles"`
	PendingArticles     int        `json:"pending_articles"`
	QueuedArticles      int        `json:"queued_articles"`
	RunningArticles     int        `json:"running_articles"`
	SucceededArticles   int        `json:"succeeded_articles"`
	FailedArticles      int        `json:"failed_articles"`
	CreatedBy           string     `json:"created_by"`
	ErrorDetail         *string    `json:"error_detail"`
	CreatedAt           time.Time  `json:"created_at"`
	StartedAt           *time.Time `json:"started_at"`
	FinishedAt          *time.Time `json:"finished_at"`
	LatestItemUpdatedAt *time.Time `json:"latest_item_updated_at"`
}

func (store *Store) Preview(ctx context.Context, request CreateRequest) (Preview, error) {
	if err := validatePreviewRequest(request); err != nil {
		return Preview{}, err
	}
	predicate, arguments := scopePredicate(request, 1)
	query := eligibleArticlesSQL + " AND " + predicate
	var count int
	if err := store.pool.QueryRow(ctx, "SELECT count(*) FROM ("+query+") eligible", arguments...).Scan(&count); err != nil {
		return Preview{}, fmt.Errorf("count eligible backfill articles: %w", err)
	}
	return Preview{Articles: count}, nil
}

func validatePreviewRequest(request CreateRequest) error {
	copy := request
	if copy.Scope == "catalog" {
		copy.Confirmation = catalogConfirmation
	}
	return ValidateCreateRequest(copy)
}

const eligibleArticlesSQL = `
	SELECT article.id
	FROM articles article
	JOIN source_compliance_reviews compliance
	  ON compliance.source_id = article.source_id AND compliance.active
	JOIN article_contents content
	  ON content.article_id = article.id AND content.current
	WHERE compliance.status IN ('approved', 'restricted')
	  AND compliance.allow_ai_processing
`

func scopePredicate(request CreateRequest, firstParameter int) (string, []any) {
	switch request.Scope {
	case "date_range":
		return fmt.Sprintf("article.published_at >= $%d AND article.published_at < $%d", firstParameter, firstParameter+1), []any{request.From, request.To}
	case "article":
		return fmt.Sprintf("article.id = $%d::uuid", firstParameter), []any{request.ArticleID}
	default:
		return "true", nil
	}
}

func (store *Store) Create(ctx context.Context, request CreateRequest, createdBy string) (Run, error) {
	if err := ValidateCreateRequest(request); err != nil {
		return Run{}, err
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return Run{}, fmt.Errorf("begin analysis backfill: %w", err)
	}
	defer tx.Rollback(ctx)

	var runID string
	err = tx.QueryRow(ctx, `
		INSERT INTO admin_analysis_backfills (
		  scope, workflow, provider, model, from_at, to_at, article_id, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, '')::uuid, $8)
		RETURNING id::text
	`, request.Scope, normalizeWorkflow(request.Workflow), request.Provider, request.Model,
		request.From, request.To, request.ArticleID, createdBy).Scan(&runID)
	if err != nil {
		return Run{}, fmt.Errorf("create analysis backfill: %w", err)
	}

	predicate, arguments := scopePredicate(request, 2)
	insertArguments := append([]any{runID}, arguments...)
	tag, err := tx.Exec(ctx, `
		INSERT INTO admin_analysis_backfill_items (run_id, article_id)
		SELECT $1::uuid, eligible.id
		FROM (`+eligibleArticlesSQL+` AND `+predicate+`) eligible
		ON CONFLICT DO NOTHING
	`, insertArguments...)
	if err != nil {
		return Run{}, fmt.Errorf("select analysis backfill articles: %w", err)
	}
	total := int(tag.RowsAffected())
	if total == 0 {
		return Run{}, fmt.Errorf("no eligible articles have approved full content in this scope")
	}
	if _, err := tx.Exec(ctx, `
		UPDATE admin_analysis_backfills SET total_articles = $2, updated_at = clock_timestamp()
		WHERE id = $1
	`, runID, total); err != nil {
		return Run{}, fmt.Errorf("update analysis backfill size: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Run{}, fmt.Errorf("commit analysis backfill: %w", err)
	}
	return store.Get(ctx, runID)
}

const runSelect = `
	SELECT run.id::text, run.scope, run.workflow, run.provider, run.model, run.from_at, run.to_at,
	       run.article_id::text, run.status, run.total_articles,
	       count(item.article_id) FILTER (WHERE item.state = 'pending')::integer,
	       count(item.article_id) FILTER (WHERE item.state = 'queued')::integer,
	       count(item.article_id) FILTER (WHERE item.state = 'running')::integer,
	       count(item.article_id) FILTER (WHERE item.state = 'succeeded')::integer,
	       count(item.article_id) FILTER (WHERE item.state = 'failed')::integer,
	       run.created_by, run.error_detail, run.created_at, run.started_at, run.finished_at,
	       max(item.updated_at)
	FROM admin_analysis_backfills run
	LEFT JOIN admin_analysis_backfill_items item ON item.run_id = run.id
`

const runGroup = `
	GROUP BY run.id, run.scope, run.workflow, run.provider, run.model, run.from_at, run.to_at,
	         run.article_id, run.status, run.total_articles, run.created_by,
	         run.error_detail, run.created_at, run.started_at, run.finished_at
`

func scanRun(row pgx.Row) (Run, error) {
	var run Run
	err := row.Scan(
		&run.ID, &run.Scope, &run.Workflow, &run.Provider, &run.Model, &run.From, &run.To, &run.ArticleID,
		&run.Status, &run.TotalArticles, &run.PendingArticles, &run.QueuedArticles,
		&run.RunningArticles, &run.SucceededArticles, &run.FailedArticles, &run.CreatedBy,
		&run.ErrorDetail, &run.CreatedAt, &run.StartedAt, &run.FinishedAt, &run.LatestItemUpdatedAt,
	)
	return run, err
}

func (store *Store) Get(ctx context.Context, id string) (Run, error) {
	run, err := scanRun(store.pool.QueryRow(ctx, runSelect+" WHERE run.id = $1 "+runGroup, id))
	if err != nil {
		return Run{}, fmt.Errorf("get analysis backfill: %w", err)
	}
	return run, nil
}

func (store *Store) List(ctx context.Context, limit int) ([]Run, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := store.pool.Query(ctx, runSelect+runGroup+" ORDER BY run.created_at DESC LIMIT $1", limit)
	if err != nil {
		return nil, fmt.Errorf("list analysis backfills: %w", err)
	}
	defer rows.Close()
	items := make([]Run, 0, limit)
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan analysis backfill: %w", err)
		}
		items = append(items, run)
	}
	return items, rows.Err()
}

func (store *Store) PendingArticles(ctx context.Context, runID string, limit int) ([]string, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT article_id::text
		FROM admin_analysis_backfill_items
		WHERE run_id = $1 AND state = 'pending'
		ORDER BY article_id
		LIMIT $2
	`, runID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]string, 0, limit)
	for rows.Next() {
		var articleID string
		if err := rows.Scan(&articleID); err != nil {
			return nil, err
		}
		items = append(items, articleID)
	}
	return items, rows.Err()
}

func (store *Store) MarkRunStarted(ctx context.Context, runID string) error {
	_, err := store.pool.Exec(ctx, `
		UPDATE admin_analysis_backfills
		SET status = 'running', started_at = COALESCE(started_at, clock_timestamp()), updated_at = clock_timestamp()
		WHERE id = $1 AND status = 'queued'
	`, runID)
	return err
}

func (store *Store) MarkRunFailed(ctx context.Context, runID, detail string) error {
	_, err := store.pool.Exec(ctx, `
		UPDATE admin_analysis_backfills
		SET status = 'failed', error_detail = $2, finished_at = clock_timestamp(), updated_at = clock_timestamp()
		WHERE id = $1
	`, runID, truncateRunes(detail, 2000))
	return err
}

func (store *Store) MarkQueued(ctx context.Context, runID, articleID string, jobID int64) error {
	_, err := store.pool.Exec(ctx, `
		UPDATE admin_analysis_backfill_items
		SET state = 'queued', river_job_id = $3, queued_at = COALESCE(queued_at, clock_timestamp()),
		    updated_at = clock_timestamp()
		WHERE run_id = $1 AND article_id = $2 AND state = 'pending'
	`, runID, articleID, jobID)
	return err
}

func (store *Store) MarkRunning(ctx context.Context, runID, articleID string, attempt int) error {
	_, err := store.pool.Exec(ctx, `
		UPDATE admin_analysis_backfill_items
		SET state = 'running', attempt = $3, started_at = COALESCE(started_at, clock_timestamp()),
		    error_detail = NULL, updated_at = clock_timestamp()
		WHERE run_id = $1 AND article_id = $2
	`, runID, articleID, attempt)
	return err
}

func (store *Store) MarkAttemptFailed(ctx context.Context, runID, articleID, detail string, terminal bool) error {
	state := "queued"
	if terminal {
		state = "failed"
	}
	_, err := store.pool.Exec(ctx, `
		UPDATE admin_analysis_backfill_items
		SET state = $3, error_detail = $4,
		    finished_at = CASE WHEN $3 = 'failed' THEN clock_timestamp() ELSE NULL END,
		    updated_at = clock_timestamp()
		WHERE run_id = $1 AND article_id = $2
	`, runID, articleID, state, truncateRunes(detail, 2000))
	if err == nil && terminal {
		err = store.RefreshRunStatus(ctx, runID)
	}
	return err
}

func (store *Store) SaveInsight(ctx context.Context, runID, articleID, provider, model string, insight Insight) error {
	evidence, err := json.Marshal(insight.Evidence)
	if err != nil {
		return err
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO article_ai_insights (
		  article_id, backfill_run_id, summary, tone, political_relevance,
		  political_narrative, spectrum_score, confidence, evidence, provider, provider_model
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (article_id) DO UPDATE SET
		  backfill_run_id = EXCLUDED.backfill_run_id, summary = EXCLUDED.summary,
		  tone = EXCLUDED.tone, political_relevance = EXCLUDED.political_relevance,
		  political_narrative = EXCLUDED.political_narrative,
		  spectrum_score = EXCLUDED.spectrum_score, confidence = EXCLUDED.confidence,
		  evidence = EXCLUDED.evidence, provider = EXCLUDED.provider,
		  provider_model = EXCLUDED.provider_model, analyzed_at = clock_timestamp()
	`, articleID, runID, insight.Summary, insight.Tone, insight.PoliticalRelevant,
		insight.PoliticalNarrative, insight.SpectrumScore, insight.Confidence, evidence, provider, model); err != nil {
		return fmt.Errorf("save article insight: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE admin_analysis_backfill_items
		SET state = 'succeeded', error_detail = NULL, finished_at = clock_timestamp(), updated_at = clock_timestamp()
		WHERE run_id = $1 AND article_id = $2
	`, runID, articleID); err != nil {
		return fmt.Errorf("complete analysis backfill item: %w", err)
	}
	if _, err := tx.Exec(ctx, refreshRunStatusSQL, runID); err != nil {
		return fmt.Errorf("refresh analysis backfill progress: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

// MarkSucceeded completes a tracked backfill item whose workflow persisted its
// results through the regular editorial pipeline instead of article_ai_insights.
func (store *Store) MarkSucceeded(ctx context.Context, runID, articleID string) error {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin backfill completion: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		UPDATE admin_analysis_backfill_items
		SET state = 'succeeded', error_detail = NULL, finished_at = clock_timestamp(),
		    updated_at = clock_timestamp()
		WHERE run_id = $1 AND article_id = $2
	`, runID, articleID); err != nil {
		return fmt.Errorf("complete analysis backfill item: %w", err)
	}
	if _, err := tx.Exec(ctx, refreshRunStatusSQL, runID); err != nil {
		return fmt.Errorf("refresh analysis backfill progress: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit backfill completion: %w", err)
	}
	return nil
}

func (store *Store) RefreshRunStatus(ctx context.Context, runID string) error {
	_, err := store.pool.Exec(ctx, refreshRunStatusSQL, runID)
	return err
}

const refreshRunStatusSQL = `
	WITH progress AS (
	  SELECT count(*) FILTER (WHERE state = 'succeeded')::integer AS succeeded,
	         count(*) FILTER (WHERE state = 'failed')::integer AS failed,
	         count(*)::integer AS total
	  FROM admin_analysis_backfill_items WHERE run_id = $1
	)
	UPDATE admin_analysis_backfills run
	SET status = CASE
	      WHEN progress.succeeded + progress.failed < progress.total THEN 'running'
	      WHEN progress.failed = 0 THEN 'completed'
	      WHEN progress.succeeded = 0 THEN 'failed'
	      ELSE 'partially_completed'
	    END,
	    finished_at = CASE WHEN progress.succeeded + progress.failed = progress.total THEN clock_timestamp() ELSE NULL END,
	    updated_at = clock_timestamp()
	FROM progress WHERE run.id = $1
`

func (store *Store) Article(ctx context.Context, articleID string) (ArticleInput, error) {
	var article ArticleInput
	err := store.pool.QueryRow(ctx, `
		SELECT article.headline, COALESCE(article.description, ''), content.body_text
		FROM articles article
		JOIN article_contents content ON content.article_id = article.id AND content.current
		JOIN source_compliance_reviews compliance
		  ON compliance.source_id = article.source_id AND compliance.active
		WHERE article.id = $1
		  AND compliance.status IN ('approved', 'restricted')
		  AND compliance.allow_ai_processing
	`, articleID).Scan(&article.Headline, &article.Description, &article.Body)
	if err != nil {
		return ArticleInput{}, fmt.Errorf("load approved full article: %w", err)
	}
	return article, nil
}
