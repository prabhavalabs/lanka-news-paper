// Package retention deletes operational logs older than the configured retention period:
// crawl attempts, ingestion runs, pipeline history and logs, model-call logs and log files.
package retention

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DefaultDays = 7
	MinDays     = 1
	MaxDays     = 365
	// Each delete statement removes at most this many rows so no transaction holds locks for long.
	batchSize = 5000
)

var ErrInvalid = errors.New("retention days must be between 1 and 365")

type Settings struct {
	RetentionDays  int              `json:"retention_days"`
	LastRunAt      *time.Time       `json:"last_run_at"`
	LastRunSummary map[string]int64 `json:"last_run_summary"`
	LastRunError   *string          `json:"last_run_error"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

// HistoryDeleter removes finished pipeline runs (with their steps, logs and model calls).
type HistoryDeleter interface {
	DeleteHistory(context.Context, time.Time) (int64, error)
}

type Store struct {
	pool     *pgxpool.Pool
	history  HistoryDeleter
	logPaths []string
}

// NewStore prunes the database through pool and, on each run, files older than the retention
// period under logPaths. history may be nil.
func NewStore(pool *pgxpool.Pool, history HistoryDeleter, logPaths ...string) *Store {
	return &Store{pool: pool, history: history, logPaths: logPaths}
}

func (store *Store) Settings(ctx context.Context) (Settings, error) {
	var settings Settings
	var summary []byte
	err := store.pool.QueryRow(ctx, `
		SELECT retention_days, last_run_at, last_run_summary, last_run_error, updated_at
		FROM log_retention_settings WHERE singleton
	`).Scan(&settings.RetentionDays, &settings.LastRunAt, &summary, &settings.LastRunError, &settings.UpdatedAt)
	if err != nil {
		return Settings{}, fmt.Errorf("load log retention settings: %w", err)
	}
	settings.LastRunSummary = map[string]int64{}
	if err := json.Unmarshal(summary, &settings.LastRunSummary); err != nil {
		return Settings{}, fmt.Errorf("decode log retention summary: %w", err)
	}
	return settings, nil
}

func (store *Store) UpdateSettings(ctx context.Context, actor uuid.UUID, days int) (Settings, error) {
	if days < MinDays || days > MaxDays {
		return Settings{}, ErrInvalid
	}
	var updatedBy *uuid.UUID
	if actor != uuid.Nil {
		updatedBy = &actor
	}
	if _, err := store.pool.Exec(ctx, `
		UPDATE log_retention_settings
		SET retention_days = $1, updated_by = $2, updated_at = clock_timestamp()
		WHERE singleton
	`, days, updatedBy); err != nil {
		return Settings{}, fmt.Errorf("update log retention settings: %w", err)
	}
	return store.Settings(ctx)
}

// Run deletes everything older than the retention period and records the outcome.
func (store *Store) Run(ctx context.Context, now time.Time) (map[string]int64, error) {
	settings, err := store.Settings(ctx)
	if err != nil {
		return nil, err
	}
	cutoff := now.Add(-time.Duration(settings.RetentionDays) * 24 * time.Hour)
	summary, runErr := store.cleanup(ctx, cutoff)
	var message *string
	if runErr != nil {
		text := runErr.Error()
		message = &text
	}
	encoded, err := json.Marshal(summary)
	if err != nil {
		return summary, errors.Join(runErr, err)
	}
	if _, err := store.pool.Exec(ctx, `
		UPDATE log_retention_settings
		SET last_run_at = $1, last_run_summary = $2, last_run_error = $3
		WHERE singleton
	`, now, encoded, message); err != nil {
		return summary, errors.Join(runErr, fmt.Errorf("record log retention run: %w", err))
	}
	return summary, runErr
}

func (store *Store) cleanup(ctx context.Context, cutoff time.Time) (map[string]int64, error) {
	summary := map[string]int64{}
	if store.history != nil {
		deleted, err := store.history.DeleteHistory(ctx, cutoff)
		summary["pipeline_runs"] = deleted
		if err != nil {
			return summary, err
		}
	}
	targets := []struct{ key, table, condition string }{
		{"pipeline_logs", "article_pipeline_logs", "created_at < $1"},
		{"model_calls", "llm_calls", "created_at < $1"},
		{"ingestion_runs", "ingestion_runs", "started_at < $1 AND status <> 'running'"},
		// Failed fetches for articles that never got a body are kept: they stop the crawler
		// from retrying an article that already failed three times.
		{"crawl_attempts", "crawl_attempts", `started_at < $1 AND status <> 'running' AND NOT (
			status IN ('failed', 'blocked') AND article_id IS NOT NULL
			AND NOT EXISTS (SELECT 1 FROM article_contents body WHERE body.article_id = crawl_attempts.article_id AND body.current))`},
	}
	for _, target := range targets {
		deleted, err := store.deleteInBatches(ctx, target.table, target.condition, cutoff)
		summary[target.key] = deleted
		if err != nil {
			return summary, fmt.Errorf("delete expired %s: %w", target.key, err)
		}
	}
	files, err := deleteOldFiles(store.logPaths, cutoff)
	summary["log_files"] = files
	return summary, err
}

func (store *Store) deleteInBatches(ctx context.Context, table, condition string, cutoff time.Time) (int64, error) {
	var total int64
	query := fmt.Sprintf(`DELETE FROM %[1]s WHERE ctid = ANY(ARRAY(SELECT ctid FROM %[1]s WHERE %[2]s LIMIT %[3]d))`, table, condition, batchSize)
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		tag, err := store.pool.Exec(ctx, query, cutoff)
		if err != nil {
			return total, err
		}
		total += tag.RowsAffected()
		if tag.RowsAffected() < batchSize {
			return total, nil
		}
	}
}

// deleteOldFiles removes regular files last modified before cutoff under each root. Missing
// roots are skipped; directories are left in place.
func deleteOldFiles(roots []string, cutoff time.Time) (int64, error) {
	var deleted int64
	var failures []error
	for _, root := range roots {
		if root == "" {
			continue
		}
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
			if !entry.Type().IsRegular() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return nil
			}
			if info.ModTime().Before(cutoff) {
				if err := os.Remove(path); err != nil {
					failures = append(failures, err)
					return nil
				}
				deleted++
			}
			return nil
		})
		if err != nil {
			failures = append(failures, fmt.Errorf("prune log files in %s: %w", root, err))
		}
	}
	return deleted, errors.Join(failures...)
}
