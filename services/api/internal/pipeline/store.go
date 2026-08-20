package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/classify"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/cluster"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/politics"
)

var steps = []string{"categorization", "event_clustering", "narration_analysis"}

const classificationPrompt = `Classify this Sri Lankan news article into exactly one slug:
latest, politics, economy, world, crime, health, environment, sport, education, entertainment, technology.
Use the article meaning, not isolated keywords. Return only the lowercase slug and nothing else.`

type Store struct {
	pool     *pgxpool.Pool
	model    *llm.Gateway
	clusters *cluster.Store
	politics *politics.Store
}

func NewStore(pool *pgxpool.Pool, model *llm.Gateway, clusters *cluster.Store, politicsStore *politics.Store) *Store {
	return &Store{pool: pool, model: model, clusters: clusters, politics: politicsStore}
}

func (store *Store) Start(ctx context.Context, articleID, trigger string) (string, error) {
	return store.start(ctx, articleID, trigger, "", false)
}

func (store *Store) Run(ctx context.Context, articleID, stepName string) (string, error) {
	if !validStep(stepName) {
		return "", fmt.Errorf("unknown pipeline step %q", stepName)
	}
	return store.start(ctx, articleID, "manual", stepName, true)
}

func validStep(name string) bool {
	if name == "" {
		return true
	}
	for _, stepName := range steps {
		if name == stepName {
			return true
		}
	}
	return false
}

func (store *Store) start(ctx context.Context, articleID, trigger, selectedStep string, rejectActive bool) (string, error) {
	if trigger == "" {
		trigger = "ingestion"
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var runID string
	insertRun := `
		INSERT INTO article_pipeline_runs (article_id, trigger)
		VALUES ($1, $2)
		ON CONFLICT (article_id) WHERE status IN ('queued', 'running')
		DO UPDATE SET updated_at = article_pipeline_runs.updated_at
		RETURNING id::text
	`
	if rejectActive {
		insertRun = `
			INSERT INTO article_pipeline_runs (article_id, trigger)
			VALUES ($1, $2)
			ON CONFLICT (article_id) WHERE status IN ('queued', 'running') DO NOTHING
			RETURNING id::text
		`
	}
	err = tx.QueryRow(ctx, insertRun, articleID, trigger).Scan(&runID)
	if rejectActive && err == pgx.ErrNoRows {
		return "", fmt.Errorf("article pipeline is already queued or running")
	}
	if err != nil {
		return "", fmt.Errorf("create article pipeline: %w", err)
	}
	for position, name := range steps {
		if _, err := tx.Exec(ctx, `
			INSERT INTO article_pipeline_steps (run_id, name, position)
			VALUES ($1, $2, $3) ON CONFLICT (run_id, name) DO NOTHING
		`, runID, name, position+1); err != nil {
			return "", fmt.Errorf("create pipeline step %s: %w", name, err)
		}
	}
	if selectedStep != "" {
		if _, err := tx.Exec(ctx, `
			UPDATE article_pipeline_steps
			SET status = 'skipped', finished_at = clock_timestamp(), duration_ms = 0,
			    output = jsonb_build_object('reason', 'Not selected for this manual run.')
			WHERE run_id = $1 AND name <> $2
		`, runID, selectedStep); err != nil {
			return "", fmt.Errorf("select pipeline step: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO article_pipeline_logs (run_id, step_id, event, message, details)
		SELECT $1, id,
		       CASE WHEN $3::text = '' OR name = $3 THEN 'step_queued' ELSE 'step_skipped' END,
		       CASE WHEN $3::text = '' OR name = $3 THEN 'Step queued' ELSE 'Step not selected' END,
		       jsonb_build_object('position', position, 'trigger', $2::text,
		                          'selected_step', NULLIF($3::text, ''))
		FROM article_pipeline_steps step
		WHERE step.run_id = $1
		  AND NOT EXISTS (
		    SELECT 1 FROM article_pipeline_logs log
		    WHERE log.step_id = step.id AND log.event = 'step_queued'
		  )
		ORDER BY step.position
	`, runID, trigger, selectedStep); err != nil {
		return "", fmt.Errorf("log pipeline creation: %w", err)
	}
	return runID, tx.Commit(ctx)
}

func (store *Store) EnsureBacklog(ctx context.Context, limit int) error {
	rows, err := store.pool.Query(ctx, `
		SELECT a.id::text
		FROM articles a
		WHERE NOT EXISTS (SELECT 1 FROM article_pipeline_runs run WHERE run.article_id = a.id)
		  AND (to_jsonb(a)->>'pipeline_enqueued_at') IS NULL
		ORDER BY a.received_at DESC, a.id
		LIMIT $1
	`, limit)
	if err != nil {
		return err
	}
	var articleIDs []string
	for rows.Next() {
		var articleID string
		if err := rows.Scan(&articleID); err != nil {
			rows.Close()
			return err
		}
		articleIDs = append(articleIDs, articleID)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, articleID := range articleIDs {
		if _, err := store.Start(ctx, articleID, "backfill"); err != nil {
			return err
		}
	}
	return nil
}

func (store *Store) QueuedRuns(ctx context.Context, limit int) ([]string, error) {
	_, _ = store.pool.Exec(ctx, `
		WITH stale AS (
		  UPDATE article_pipeline_runs
		  SET status = 'queued', last_error = 'Recovered after an interrupted worker.', updated_at = clock_timestamp()
		  WHERE status = 'running' AND updated_at < clock_timestamp() - interval '20 minutes'
		  RETURNING id
		)
		UPDATE article_pipeline_steps step
		SET status = 'queued', error_detail = 'Recovered after an interrupted worker.'
		FROM stale WHERE step.run_id = stale.id AND step.status = 'running'
	`)
	rows, err := store.pool.Query(ctx, `
		SELECT id::text FROM article_pipeline_runs
		WHERE status = 'queued' ORDER BY created_at LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0, limit)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (store *Store) DeleteHistory(ctx context.Context, before time.Time) (int64, error) {
	var ready bool
	if err := store.pool.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM information_schema.columns
		  WHERE table_schema = 'public' AND table_name = 'articles' AND column_name = 'pipeline_enqueued_at'
		)
	`).Scan(&ready); err != nil {
		return 0, fmt.Errorf("check pipeline history retention readiness: %w", err)
	}
	if !ready {
		return 0, nil
	}

	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin pipeline history cleanup: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		DELETE FROM llm_calls
		WHERE pipeline_run_id IN (
		  SELECT id FROM article_pipeline_runs
		  WHERE status IN ('succeeded', 'failed') AND finished_at < $1
		)
	`, before); err != nil {
		return 0, fmt.Errorf("delete expired pipeline model calls: %w", err)
	}
	tag, err := tx.Exec(ctx, `
		DELETE FROM article_pipeline_runs
		WHERE status IN ('succeeded', 'failed') AND finished_at < $1
	`, before)
	if err != nil {
		return 0, fmt.Errorf("delete expired pipeline runs: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit pipeline history cleanup: %w", err)
	}
	return tag.RowsAffected(), nil
}

type step struct {
	id, name, status               string
	position, attempt, maxAttempts int
}

type stepResult struct {
	output  any
	skipped bool
}

func (store *Store) Process(ctx context.Context, runID string) error {
	var articleID string
	if err := store.pool.QueryRow(ctx, `
		UPDATE article_pipeline_runs
		SET status = 'running', attempt = attempt + 1, started_at = COALESCE(started_at, clock_timestamp()),
		    finished_at = NULL, last_error = NULL, updated_at = clock_timestamp()
		WHERE id = $1 AND status IN ('queued', 'running', 'failed')
		RETURNING article_id::text
	`, runID).Scan(&articleID); err != nil {
		return fmt.Errorf("start pipeline run %s: %w", runID, err)
	}

	rows, err := store.pool.Query(ctx, `
		SELECT id::text, name, position, status, attempt, max_attempts
		FROM article_pipeline_steps WHERE run_id = $1 ORDER BY position
	`, runID)
	if err != nil {
		return err
	}
	var pending []step
	for rows.Next() {
		var item step
		if err := rows.Scan(&item.id, &item.name, &item.position, &item.status, &item.attempt, &item.maxAttempts); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}

	for _, item := range pending {
		if item.status == "succeeded" || item.status == "skipped" {
			continue
		}
		if item.attempt >= item.maxAttempts {
			return fmt.Errorf("pipeline step %s exhausted %d attempts", item.name, item.maxAttempts)
		}
		if _, err := store.pool.Exec(ctx, `
			UPDATE article_pipeline_steps
			SET status = 'running', attempt = attempt + 1, started_at = clock_timestamp(),
			    finished_at = NULL, duration_ms = NULL, error_detail = NULL
			WHERE id = $1
		`, item.id); err != nil {
			return err
		}
		_, _ = store.pool.Exec(ctx, `
			UPDATE article_pipeline_runs SET current_step = $2, updated_at = clock_timestamp() WHERE id = $1
		`, runID, item.name)
		store.appendLog(ctx, runID, item.id, "info", "step_started", "Step started", map[string]any{
			"attempt": item.attempt + 1, "max_attempts": item.maxAttempts,
		})

		result, err := store.execute(ctx, articleID, runID, item)
		if err != nil {
			detail := truncate(err.Error(), 2000)
			_, _ = store.pool.Exec(ctx, `
				UPDATE article_pipeline_steps
				SET status = 'failed', finished_at = clock_timestamp(),
				    duration_ms = GREATEST(0, (extract(epoch FROM clock_timestamp() - started_at) * 1000)::bigint),
				    error_detail = $2
				WHERE id = $1
			`, item.id, detail)
			_, _ = store.pool.Exec(ctx, `
				UPDATE article_pipeline_runs
				SET status = 'failed', last_error = $2, finished_at = clock_timestamp(), updated_at = clock_timestamp()
				WHERE id = $1
			`, runID, detail)
			store.appendLog(ctx, runID, item.id, "error", "step_failed", "Step failed", map[string]any{
				"attempt": item.attempt + 1, "error": detail,
			})
			return err
		}
		output, err := json.Marshal(result.output)
		if err != nil {
			return err
		}
		status := "succeeded"
		if result.skipped {
			status = "skipped"
		}
		if _, err := store.pool.Exec(ctx, `
			UPDATE article_pipeline_steps
			SET status = $2, finished_at = clock_timestamp(),
			    duration_ms = GREATEST(0, (extract(epoch FROM clock_timestamp() - started_at) * 1000)::bigint),
			    output = $3
			WHERE id = $1
		`, item.id, status, output); err != nil {
			return err
		}
		message := "Step completed"
		if result.skipped {
			message = "Step skipped"
		}
		store.appendLog(ctx, runID, item.id, "info", "step_"+status, message, map[string]any{
			"attempt": item.attempt + 1, "output": result.output,
		})
	}

	_, err = store.pool.Exec(ctx, `
		UPDATE article_pipeline_runs
		SET status = 'succeeded', current_step = NULL, last_error = NULL,
		    finished_at = clock_timestamp(), updated_at = clock_timestamp()
		WHERE id = $1
	`, runID)
	return err
}

func (store *Store) execute(ctx context.Context, articleID, runID string, item step) (stepResult, error) {
	switch item.name {
	case "categorization":
		return store.categorize(ctx, articleID, runID, item.id)
	case "event_clustering":
		return store.cluster(ctx, articleID)
	case "narration_analysis":
		result, err := store.politics.AnalyzeArticle(ctx, articleID, runID, item.id)
		return stepResult{output: result}, err
	default:
		return stepResult{}, fmt.Errorf("unknown pipeline step %q", item.name)
	}
}

func (store *Store) categorize(ctx context.Context, articleID, runID, stepID string) (stepResult, error) {
	var publisherCategory, headline, description string
	if err := store.pool.QueryRow(ctx, `
		SELECT COALESCE(publisher_category, ''), headline, COALESCE(description, '')
		FROM articles WHERE id = $1
	`, articleID).Scan(&publisherCategory, &headline, &description); err != nil {
		return stepResult{}, err
	}
	categories := []string(nil)
	if publisherCategory != "" {
		categories = []string{publisherCategory}
	}
	result := classify.From(categories, headline, description)
	provider, model := "keyword-rules", result.Model
	if result.Confidence < 0.55 && store.model != nil {
		response, err := store.model.Complete(ctx, llm.Request{
			Task: "classify", System: classificationPrompt,
			Input:            headline + "\n\n" + truncate(description, 3000),
			DisableReasoning: true, MaxTokens: 64, ArticleID: articleID,
			PipelineRunID: runID, PipelineStepID: stepID,
		})
		if err != nil {
			return stepResult{}, err
		}
		slug, valid := completionSlug(response.Text)
		if response.Provider != "none" && valid {
			result.Slug, result.Confidence = slug, 0.7
			provider, model = response.Provider, response.Model
		}
	}
	tag, err := store.pool.Exec(ctx, `
		UPDATE articles
		SET category_id = (SELECT id FROM categories WHERE slug = $2),
		    classify_confidence = $3, classify_model = $4
		WHERE id = $1
	`, articleID, result.Slug, result.Confidence, "pipeline:"+model)
	if err != nil || tag.RowsAffected() == 0 {
		return stepResult{}, fmt.Errorf("save category: %w", err)
	}
	return stepResult{output: map[string]any{
		"category": result.Slug, "confidence": result.Confidence, "provider": provider, "model": model,
	}}, nil
}

func completionSlug(value string) (string, bool) {
	slug := strings.Trim(strings.ToLower(value), " \n\t`\"'.")
	return slug, classify.ValidSlug(slug)
}

func (store *Store) cluster(ctx context.Context, articleID string) (stepResult, error) {
	var status string
	var eventID *string
	if err := store.pool.QueryRow(ctx, `SELECT public_status, event_id::text FROM articles WHERE id = $1`, articleID).Scan(&status, &eventID); err != nil {
		return stepResult{}, err
	}
	if status != "published" {
		return stepResult{output: map[string]any{"reason": "Article is not published."}, skipped: true}, nil
	}
	if eventID == nil {
		if err := store.clusters.Attach(ctx, articleID); err != nil {
			return stepResult{}, err
		}
		if err := store.pool.QueryRow(ctx, `SELECT event_id::text FROM articles WHERE id = $1`, articleID).Scan(&eventID); err != nil {
			return stepResult{}, err
		}
	}
	return stepResult{output: map[string]any{"event_id": eventID}}, nil
}

func (store *Store) appendLog(ctx context.Context, runID, stepID, level, event, message string, details any) {
	payload, err := json.Marshal(details)
	if err != nil {
		return
	}
	_, _ = store.pool.Exec(ctx, `
		INSERT INTO article_pipeline_logs (run_id, step_id, level, event, message, details)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, runID, stepID, level, event, message, payload)
}

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
