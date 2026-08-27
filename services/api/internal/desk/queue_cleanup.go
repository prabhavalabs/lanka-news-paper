package desk

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

const (
	QueueCleanupFailed = "failed"
	QueueCleanupAll    = "all"
)

var (
	ErrInvalidQueueCleanupScope        = errors.New("queue cleanup scope must be failed or all")
	ErrInvalidQueueCleanupConfirmation = errors.New("queue cleanup confirmation does not match")
)

type QueueCleanupPreview struct {
	Scope           string `json:"scope"`
	Confirmation    string `json:"confirmation"`
	RiverJobs       int    `json:"river_jobs"`
	PipelineRuns    int    `json:"pipeline_runs"`
	PipelineSteps   int    `json:"pipeline_steps"`
	PipelineLogs    int    `json:"pipeline_logs"`
	TotalRecords    int    `json:"total_records"`
	ActiveProtected bool   `json:"active_jobs_protected"`
}

func queueCleanupConfirmation(scope string) (string, error) {
	switch scope {
	case QueueCleanupFailed:
		return "DELETE FAILED JOBS", nil
	case QueueCleanupAll:
		return "DELETE QUEUE HISTORY", nil
	default:
		return "", ErrInvalidQueueCleanupScope
	}
}

func queueCleanupPredicates(scope string) (riverPredicate, pipelinePredicate string, err error) {
	switch scope {
	case QueueCleanupFailed:
		return "state IN ('discarded', 'cancelled')", "status = 'failed'", nil
	case QueueCleanupAll:
		return "state IN ('completed', 'discarded', 'cancelled')", "status IN ('succeeded', 'failed')", nil
	default:
		return "", "", ErrInvalidQueueCleanupScope
	}
}

func (store *Store) PreviewQueueCleanup(ctx context.Context, scope string) (QueueCleanupPreview, error) {
	confirmation, err := queueCleanupConfirmation(scope)
	if err != nil {
		return QueueCleanupPreview{}, err
	}
	riverPredicate, pipelinePredicate, _ := queueCleanupPredicates(scope)
	preview := QueueCleanupPreview{Scope: scope, Confirmation: confirmation, ActiveProtected: true}
	query := fmt.Sprintf(`
		SELECT
		  (SELECT count(*)::integer FROM river_job WHERE %s),
		  (SELECT count(*)::integer FROM article_pipeline_runs WHERE %s),
		  (SELECT count(*)::integer FROM article_pipeline_steps step
		     JOIN article_pipeline_runs run ON run.id = step.run_id WHERE run.%s),
		  (SELECT count(*)::integer FROM article_pipeline_logs log
		     JOIN article_pipeline_runs run ON run.id = log.run_id WHERE run.%s)
	`, riverPredicate, pipelinePredicate, pipelinePredicate, pipelinePredicate)
	if err := store.pool.QueryRow(ctx, query).Scan(
		&preview.RiverJobs, &preview.PipelineRuns, &preview.PipelineSteps, &preview.PipelineLogs,
	); err != nil {
		return QueueCleanupPreview{}, fmt.Errorf("preview queue cleanup: %w", err)
	}
	preview.TotalRecords = preview.RiverJobs + preview.PipelineRuns + preview.PipelineSteps + preview.PipelineLogs
	return preview, nil
}

func (store *Store) CleanupQueue(ctx context.Context, scope, confirmation string, actor uuid.UUID) (QueueCleanupPreview, error) {
	expected, err := queueCleanupConfirmation(scope)
	if err != nil {
		return QueueCleanupPreview{}, err
	}
	if confirmation != expected {
		return QueueCleanupPreview{}, ErrInvalidQueueCleanupConfirmation
	}
	riverPredicate, pipelinePredicate, _ := queueCleanupPredicates(scope)
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return QueueCleanupPreview{}, fmt.Errorf("begin queue cleanup: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if _, err := transaction.Exec(ctx, `SET LOCAL statement_timeout = '30s'`); err != nil {
		return QueueCleanupPreview{}, fmt.Errorf("set queue cleanup timeout: %w", err)
	}

	preview := QueueCleanupPreview{Scope: scope, Confirmation: expected, ActiveProtected: true}
	countQuery := fmt.Sprintf(`
		SELECT
		  (SELECT count(*)::integer FROM article_pipeline_steps step
		     JOIN article_pipeline_runs run ON run.id = step.run_id WHERE run.%s),
		  (SELECT count(*)::integer FROM article_pipeline_logs log
		     JOIN article_pipeline_runs run ON run.id = log.run_id WHERE run.%s)
	`, pipelinePredicate, pipelinePredicate)
	if err := transaction.QueryRow(ctx, countQuery).Scan(&preview.PipelineSteps, &preview.PipelineLogs); err != nil {
		return QueueCleanupPreview{}, fmt.Errorf("count dependent queue records: %w", err)
	}

	riverDelete := fmt.Sprintf("DELETE FROM river_job WHERE %s", riverPredicate)
	riverTag, err := transaction.Exec(ctx, riverDelete)
	if err != nil {
		return QueueCleanupPreview{}, fmt.Errorf("delete River queue history: %w", err)
	}
	pipelineDelete := fmt.Sprintf("DELETE FROM article_pipeline_runs WHERE %s", pipelinePredicate)
	pipelineTag, err := transaction.Exec(ctx, pipelineDelete)
	if err != nil {
		return QueueCleanupPreview{}, fmt.Errorf("delete pipeline queue history: %w", err)
	}
	preview.RiverJobs = int(riverTag.RowsAffected())
	preview.PipelineRuns = int(pipelineTag.RowsAffected())
	preview.TotalRecords = preview.RiverJobs + preview.PipelineRuns + preview.PipelineSteps + preview.PipelineLogs

	target := fmt.Sprintf("%s:river=%d,pipelines=%d,steps=%d,logs=%d",
		scope, preview.RiverJobs, preview.PipelineRuns, preview.PipelineSteps, preview.PipelineLogs)
	if _, err := transaction.Exec(ctx, `
		INSERT INTO audit_logs (actor_id, action, target_type, target_id, result)
		VALUES ($1, 'cleanup_queue_history', 'queue_history', $2, 'ok')
	`, actor, target); err != nil {
		return QueueCleanupPreview{}, fmt.Errorf("audit queue cleanup: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return QueueCleanupPreview{}, fmt.Errorf("commit queue cleanup: %w", err)
	}
	return preview, nil
}
