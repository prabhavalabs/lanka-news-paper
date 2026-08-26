package adminanalysis

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var (
	ErrRunInactive          = errors.New("administrative backfill is not running")
	ErrInvalidRunTransition = errors.New("administrative backfill cannot perform that action in its current state")
)

type ControlResult struct {
	Run    Run
	JobIDs []int64
}

func validateControlTransition(action, status string) error {
	allowed := false
	switch action {
	case "pause":
		allowed = status == "queued" || status == "running"
	case "resume":
		allowed = status == "paused"
	case "cancel":
		allowed = status == "queued" || status == "running" || status == "paused"
	case "delete":
		allowed = status == "completed" || status == "partially_completed" || status == "failed" || status == "cancelled"
	}
	if allowed {
		return nil
	}
	return fmt.Errorf("%w: cannot %s a %s run", ErrInvalidRunTransition, action, status)
}

func (store *Store) Pause(ctx context.Context, id string) (ControlResult, error) {
	return store.control(ctx, id, "pause")
}

func (store *Store) Resume(ctx context.Context, id string) (Run, error) {
	result, err := store.control(ctx, id, "resume")
	return result.Run, err
}

func (store *Store) Cancel(ctx context.Context, id string) (ControlResult, error) {
	return store.control(ctx, id, "cancel")
}

func (store *Store) Delete(ctx context.Context, id string) error {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin deleting administrative backfill: %w", err)
	}
	defer tx.Rollback(ctx)

	status, err := lockedRunStatus(ctx, tx, id)
	if err != nil {
		return err
	}
	if err := validateControlTransition("delete", status); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM admin_analysis_backfills WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete administrative backfill: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit administrative backfill deletion: %w", err)
	}
	return nil
}

func (store *Store) control(ctx context.Context, id, action string) (ControlResult, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return ControlResult{}, fmt.Errorf("begin administrative backfill control: %w", err)
	}
	defer tx.Rollback(ctx)

	status, err := lockedRunStatus(ctx, tx, id)
	if err != nil {
		return ControlResult{}, err
	}
	if err := validateControlTransition(action, status); err != nil {
		return ControlResult{}, err
	}

	jobIDs, err := activeBackfillJobIDs(ctx, tx, id)
	if err != nil {
		return ControlResult{}, err
	}

	switch action {
	case "pause":
		if _, err := tx.Exec(ctx, `
			UPDATE admin_analysis_backfills
			SET status = 'paused', finished_at = NULL, updated_at = clock_timestamp()
			WHERE id = $1
		`, id); err != nil {
			return ControlResult{}, fmt.Errorf("pause administrative backfill: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE admin_analysis_backfill_items
			SET state = 'pending', river_job_id = NULL, queued_at = NULL, started_at = NULL,
			    finished_at = NULL, error_detail = NULL, updated_at = clock_timestamp()
			WHERE run_id = $1 AND state IN ('pending', 'queued', 'running')
		`, id); err != nil {
			return ControlResult{}, fmt.Errorf("reset paused administrative backfill items: %w", err)
		}
	case "resume":
		if _, err := tx.Exec(ctx, `
			UPDATE admin_analysis_backfills
			SET status = 'queued', finished_at = NULL, error_detail = NULL, updated_at = clock_timestamp()
			WHERE id = $1
		`, id); err != nil {
			return ControlResult{}, fmt.Errorf("resume administrative backfill: %w", err)
		}
	case "cancel":
		if _, err := tx.Exec(ctx, `
			UPDATE admin_analysis_backfills
			SET status = 'cancelled', finished_at = clock_timestamp(), updated_at = clock_timestamp()
			WHERE id = $1
		`, id); err != nil {
			return ControlResult{}, fmt.Errorf("cancel administrative backfill: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE admin_analysis_backfill_items
			SET state = 'cancelled', finished_at = clock_timestamp(), updated_at = clock_timestamp()
			WHERE run_id = $1 AND state IN ('pending', 'queued', 'running')
		`, id); err != nil {
			return ControlResult{}, fmt.Errorf("cancel administrative backfill items: %w", err)
		}
	default:
		return ControlResult{}, fmt.Errorf("%w: unknown action %s", ErrInvalidRunTransition, action)
	}

	if err := tx.Commit(ctx); err != nil {
		return ControlResult{}, fmt.Errorf("commit administrative backfill control: %w", err)
	}
	run, err := store.Get(ctx, id)
	if err != nil {
		return ControlResult{}, err
	}
	return ControlResult{Run: run, JobIDs: jobIDs}, nil
}

func lockedRunStatus(ctx context.Context, tx pgx.Tx, id string) (string, error) {
	var status string
	if err := tx.QueryRow(ctx, `
		SELECT status FROM admin_analysis_backfills WHERE id = $1 FOR UPDATE
	`, id).Scan(&status); err != nil {
		return "", fmt.Errorf("lock administrative backfill: %w", err)
	}
	return status, nil
}

func activeBackfillJobIDs(ctx context.Context, tx pgx.Tx, id string) ([]int64, error) {
	rows, err := tx.Query(ctx, `
		SELECT river_job.id
		FROM river_job
		WHERE river_job.kind IN ('admin.analysis.backfill.dispatch', 'admin.article.analysis')
		  AND river_job.args->>'run_id' = $1
		  AND river_job.state IN ('available', 'pending', 'running', 'retryable', 'scheduled')
		ORDER BY river_job.id
	`, id)
	if err != nil {
		return nil, fmt.Errorf("list active administrative backfill jobs: %w", err)
	}
	defer rows.Close()
	jobIDs := make([]int64, 0)
	for rows.Next() {
		var jobID int64
		if err := rows.Scan(&jobID); err != nil {
			return nil, fmt.Errorf("scan active administrative backfill job: %w", err)
		}
		jobIDs = append(jobIDs, jobID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list active administrative backfill jobs: %w", err)
	}
	return jobIDs, nil
}
