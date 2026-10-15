package storyline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
)

type Model interface {
	Complete(context.Context, llm.Request) (llm.Response, error)
}
type Service struct {
	store *Store
	model Model
}

func NewService(store *Store, model Model) *Service { return &Service{store: store, model: model} }

// Refresh reads and calls the model without a transaction. River uniqueness limits
// scheduled calls; the short apply transaction rechecks concurrent membership edits.
func (s *Service) Refresh(ctx context.Context, window time.Duration) error {
	now := time.Now().UTC()
	if window <= 0 {
		window = 48 * time.Hour
	}
	var empty bool
	if err := s.store.pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM storylines)`).Scan(&empty); err != nil {
		return err
	}
	if empty {
		window = max(window, 7*24*time.Hour)
	}
	events, existing, terms, err := groupingInput(ctx, s.store.pool, now, window)
	if err != nil {
		return err
	}
	var groups []Group
	if len(events) > 0 && s.model != nil {
		input, _ := json.Marshal(map[string]any{"events": events, "storylines": existing, "burst_terms": terms})
		response, callErr := s.model.Complete(ctx, llm.Request{Task: "storyline_grouping", System: groupingPrompt, Input: string(input), JSONSchema: groupingSchema, MaxTokens: 6000})
		if callErr == nil && (response.Provider == "" || response.Provider == "none") {
			callErr = errors.New("storyline model unavailable")
		}
		if callErr == nil {
			groups, callErr = validateGroups(response.Text, events, existing)
		}
		if callErr != nil {
			slog.WarnContext(ctx, "storyline grouping unavailable; rescoring existing storylines", "error", callErr)
		}
	}
	tx, err := s.store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(831704266)`); err != nil {
		return err
	}
	now = time.Now().UTC()
	if err = applyGroups(ctx, tx, groups, now); err != nil {
		return fmt.Errorf("apply storyline groups: %w", err)
	}
	if err = rescore(ctx, tx, now); err != nil {
		return fmt.Errorf("rescore storylines: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	slog.InfoContext(ctx, "storylines refreshed", "candidate_events", len(events), "active_storylines", len(existing), "window_hours", window.Hours())
	return nil
}
