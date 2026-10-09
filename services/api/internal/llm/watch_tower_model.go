package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrWatchTowerModelInvalid = errors.New("invalid Watch Tower model assignment")
	ErrWatchTowerModelMissing = errors.New("Watch Tower model profiles are missing")
)

var watchTowerTasks = []string{"watch_tower_retrieval", "watch_tower_answer"}

// UpdateWatchTowerModel assigns one compatible Codex model to both Watch Tower
// stages in a transaction so retrieval and answer generation cannot drift apart.
func (gateway *Gateway) UpdateWatchTowerModel(ctx context.Context, actor uuid.UUID, providerID, modelID, effort string) error {
	providerID = strings.TrimSpace(providerID)
	modelID = strings.TrimSpace(modelID)
	for _, task := range watchTowerTasks {
		if err := validateProfile(task, providerID, modelID, effort); err != nil {
			return fmt.Errorf("%w: %v", ErrWatchTowerModelInvalid, err)
		}
	}

	transaction, err := gateway.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Watch Tower model update: %w", err)
	}
	rollback := func(cause error) error {
		if rollbackErr := transaction.Rollback(ctx); rollbackErr != nil {
			return errors.Join(cause, fmt.Errorf("roll back Watch Tower model update: %w", rollbackErr))
		}
		return cause
	}

	command, err := transaction.Exec(ctx, `
		UPDATE llm_task_profiles
		SET provider_id = $1, model = $2, reasoning_effort = NULLIF($4, ''), enabled = true
		WHERE task = ANY($3::text[])
	`, providerID, modelID, watchTowerTasks, effort)
	if err != nil {
		return rollback(fmt.Errorf("update Watch Tower model profiles: %w", err))
	}
	if command.RowsAffected() != int64(len(watchTowerTasks)) {
		return rollback(ErrWatchTowerModelMissing)
	}
	if _, err := transaction.Exec(ctx, `
		INSERT INTO audit_logs (actor_id, action, target_type, target_id, result)
		VALUES ($1, 'update_watch_tower_model', 'watch_tower_model', $2, 'ok')
	`, actor, modelID); err != nil {
		return rollback(fmt.Errorf("audit Watch Tower model update: %w", err))
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit Watch Tower model update: %w", err)
	}
	return nil
}
