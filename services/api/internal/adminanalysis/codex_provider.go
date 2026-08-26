package adminanalysis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
)

type codexCompleter interface {
	Complete(context.Context, string, string, map[string]any) (string, error)
}

// CodexProvider adapts the local Codex CLI to the same explicit-provider
// contract used by hosted LLMs. Keeping the adapter here isolates the CLI
// transport and makes the editorial pipeline provider-neutral.
type CodexProvider struct {
	store  *Store
	client codexCompleter
}

func NewCodexProvider(store *Store, client codexCompleter) *CodexProvider {
	return &CodexProvider{store: store, client: client}
}

func (*CodexProvider) ID() string { return "codex_cli" }

func (provider *CodexProvider) Complete(ctx context.Context, request llm.Request, model string) (llm.Response, error) {
	if model == "" {
		return llm.Response{}, fmt.Errorf("model is required")
	}
	started := time.Now()
	callID := provider.startCall(ctx, request, model)
	provider.logPipeline(ctx, request, "info", "provider_request_started", "Provider request started", map[string]any{
		"call_id": callID, "provider": provider.ID(), "model": model, "task": request.Task,
	})
	prompt := request.System + "\n\nInput:\n" + request.Input
	output, err := provider.client.Complete(ctx, model, prompt, request.JSONSchema)
	provider.finishCall(context.WithoutCancel(ctx), callID, time.Since(started), output, err)
	if err != nil {
		provider.logPipeline(context.WithoutCancel(ctx), request, "error", "provider_request_failed", "Provider request failed", map[string]any{
			"call_id": callID, "provider": provider.ID(), "model": model, "error": err.Error(),
		})
		return llm.Response{}, err
	}
	provider.logPipeline(context.WithoutCancel(ctx), request, "info", "provider_response_completed", "Structured response completed", map[string]any{
		"call_id": callID, "provider": provider.ID(), "model": model, "latency_ms": time.Since(started).Milliseconds(),
	})
	return llm.Response{Text: output, Provider: provider.ID(), Model: model}, nil
}

func (provider *CodexProvider) startCall(ctx context.Context, request llm.Request, model string) int64 {
	if request.PipelineStepID != "" {
		_, _ = provider.store.pool.Exec(ctx, `
			UPDATE llm_calls
			SET outcome = 'fallback', error_detail = 'Superseded by a retried provider request.',
			    latency_ms = GREATEST(0, (extract(epoch FROM clock_timestamp() - created_at) * 1000)::integer),
			    completed_at = clock_timestamp()
			WHERE pipeline_step_id = $1 AND outcome = 'running'
		`, request.PipelineStepID)
	}
	var id int64
	_ = provider.store.pool.QueryRow(ctx, `
		INSERT INTO llm_calls (
		  task, provider_id, model, outcome, article_id, pipeline_run_id, pipeline_step_id, streamed
		)
		VALUES ($1, 'codex_cli', $2, 'running', NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, NULLIF($5, '')::uuid, false)
		RETURNING id
	`, request.Task, model, request.ArticleID, request.PipelineRunID, request.PipelineStepID).Scan(&id)
	return id
}

func (provider *CodexProvider) finishCall(ctx context.Context, id int64, duration time.Duration, output string, callErr error) {
	if id == 0 {
		return
	}
	outcome, detail := "ok", ""
	if callErr != nil {
		outcome, detail = "failed", callErr.Error()
	}
	recordContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, _ = provider.store.pool.Exec(recordContext, `
		UPDATE llm_calls
		SET outcome = $2, latency_ms = $3, response_text = NULLIF($4, ''),
		    error_detail = NULLIF($5, ''), completed_at = clock_timestamp()
		WHERE id = $1
	`, id, outcome, duration.Milliseconds(), output, detail)
}

func (provider *CodexProvider) logPipeline(ctx context.Context, request llm.Request, level, event, message string, details map[string]any) {
	if request.PipelineRunID == "" {
		return
	}
	payload, err := json.Marshal(details)
	if err != nil {
		return
	}
	_, _ = provider.store.pool.Exec(ctx, `
		INSERT INTO article_pipeline_logs (run_id, step_id, level, event, message, details)
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, $6)
	`, request.PipelineRunID, request.PipelineStepID, level, event, message, payload)
}
