package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/classify"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm/codex"
)

var ErrProviderBlocked = errors.New("Codex provider is in cooldown")

type Request struct {
	Task             string
	System           string
	Input            string
	JSONSchema       map[string]any
	DisableReasoning bool
	MaxTokens        int // Retained for callers; Codex has no output-token limit option.
	ArticleID        string
	PipelineRunID    string
	PipelineStepID   string
}

type Response struct {
	CallID       int64
	Text         string
	Provider     string
	Model        string
	FinishReason string
}

type providerProfile struct {
	id, kind, model, reasoningEffort string
	timeout                          int
}

type providerResult struct {
	Text              string
	InputTokens       *int
	OutputTokens      *int
	FirstTokenMS      *int
	FinishReason      string
	CachedInputTokens int
	ReasoningTokens   int
}

type CodexClient interface {
	Complete(context.Context, codex.CallOptions) (codex.Result, error)
	Probe(context.Context) codex.Status
}

type Gateway struct {
	pool     *pgxpool.Pool
	client   CodexClient
	cooldown time.Duration
}

func NewGateway(pool *pgxpool.Pool, clients ...CodexClient) *Gateway {
	var client CodexClient = codex.NewClient(nil)
	if len(clients) > 0 && clients[0] != nil {
		client = clients[0]
	}
	cooldown, err := time.ParseDuration(os.Getenv("SNAP_CODEX_QUOTA_COOLDOWN"))
	if err != nil || cooldown <= 0 {
		cooldown = 15 * time.Minute
	}
	return &Gateway{pool: pool, client: client, cooldown: cooldown}
}

func (gateway *Gateway) Complete(ctx context.Context, request Request) (Response, error) {
	request = gateway.applyWorkflow(ctx, request)
	var profile providerProfile
	err := gateway.pool.QueryRow(ctx, `
		SELECT p.id, p.kind, t.model, t.timeout_seconds, COALESCE(t.reasoning_effort, '')
		FROM llm_task_profiles t JOIN llm_providers p ON p.id = t.provider_id
		WHERE t.task = $1 AND t.enabled AND p.enabled
	`, request.Task).Scan(&profile.id, &profile.kind, &profile.model, &profile.timeout, &profile.reasoningEffort)
	if err != nil {
		return gateway.fallback(request), nil
	}
	return gateway.completeWithProfile(ctx, request, profile, true)
}

// CompleteWithModel is strict so administrative failures remain retryable.
// The task's timeout and reasoning effort still apply to a pinned model.
func (gateway *Gateway) CompleteWithModel(ctx context.Context, request Request, providerID, model string) (Response, error) {
	providerID, model = strings.TrimSpace(providerID), strings.TrimSpace(model)
	if err := validateProfile(request.Task, providerID, model, ""); err != nil {
		return Response{}, err
	}
	request = gateway.applyWorkflow(ctx, request)
	var profile providerProfile
	err := gateway.pool.QueryRow(ctx, `
		SELECT p.id, p.kind, $2::text, t.timeout_seconds, COALESCE(t.reasoning_effort, '')
		FROM llm_providers p JOIN llm_task_profiles t ON t.provider_id = p.id
		WHERE p.id = $1 AND p.enabled AND t.task = $3
	`, providerID, model, request.Task).Scan(&profile.id, &profile.kind, &profile.model, &profile.timeout, &profile.reasoningEffort)
	if err != nil {
		return Response{}, fmt.Errorf("load explicit provider %s: %w", providerID, err)
	}
	return gateway.completeWithProfile(ctx, request, profile, false)
}

func (gateway *Gateway) completeWithProfile(ctx context.Context, request Request, profile providerProfile, allowFallback bool) (Response, error) {
	started := time.Now()
	callID := gateway.startCall(ctx, request, profile)
	gateway.logPipeline(ctx, request, "info", "provider_request_started", "Provider request started", map[string]any{
		"call_id": callID, "provider": profile.id, "model": profile.model,
		"task": request.Task, "timeout_seconds": profile.timeout, "streamed": false,
	}, started)
	var result providerResult
	blockedErr := gateway.providerBlockError(ctx)
	callErr := blockedErr
	if callErr == nil {
		result, callErr = gateway.callProvider(ctx, profile, request)
	}
	latencyMS := time.Since(started).Milliseconds()
	recordContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if callErr != nil {
		// Fast failures do not extend the cooldown; only an attempted call does.
		if blockedErr == nil && !errors.Is(callErr, ErrProviderBlocked) {
			gateway.recordCallStatus(recordContext, callErr, started)
		}
		outcome := "failed"
		if allowFallback {
			outcome = "fallback"
		}
		gateway.finishCall(recordContext, callID, outcome, latencyMS, result, callErr)
		gateway.logPipeline(recordContext, request, "error", "provider_request_failed", "Provider request failed", map[string]any{
			"call_id": callID, "provider": profile.id, "model": profile.model, "latency_ms": latencyMS, "error": callErr.Error(),
		}, time.Now())
		if allowFallback {
			response := gateway.fallback(request)
			response.CallID = callID
			return response, nil
		}
		return Response{}, callErr
	}
	gateway.recordCallStatus(recordContext, nil, started)
	gateway.finishCall(recordContext, callID, "ok", latencyMS, result, nil)
	gateway.logPipeline(recordContext, request, "info", "provider_response_completed", "Structured response completed", map[string]any{
		"call_id": callID, "provider": profile.id, "model": profile.model, "latency_ms": latencyMS,
		"input_tokens": result.InputTokens, "output_tokens": result.OutputTokens,
		"cached_input_tokens": result.CachedInputTokens, "reasoning_tokens": result.ReasoningTokens,
		"finish_reason": result.FinishReason,
	}, time.Now())
	return Response{CallID: callID, Text: result.Text, Provider: profile.id, Model: profile.model, FinishReason: result.FinishReason}, nil
}

func (gateway *Gateway) callProvider(ctx context.Context, profile providerProfile, request Request) (providerResult, error) {
	if profile.kind != "codex_cli" {
		return providerResult{}, fmt.Errorf("unsupported provider kind %q", profile.kind)
	}
	effort := profile.reasoningEffort
	if effort == "" && request.DisableReasoning {
		effort = "low"
	}
	result, err := gateway.client.Complete(ctx, codex.CallOptions{
		Model: profile.model, ReasoningEffort: effort, System: request.System, Input: request.Input,
		JSONSchema: request.JSONSchema, Timeout: time.Duration(profile.timeout) * time.Second, BeforeRun: gateway.providerBlockError,
	})
	if err != nil && result.InputTokens == 0 && result.OutputTokens == 0 {
		return providerResult{Text: result.Text}, err
	}
	finish := ""
	if err == nil {
		finish = "stop"
	}
	return providerResult{Text: result.Text, InputTokens: intPointer(result.InputTokens), OutputTokens: intPointer(result.OutputTokens),
		CachedInputTokens: result.CachedInputTokens, ReasoningTokens: result.ReasoningTokens, FirstTokenMS: result.FirstTokenMS, FinishReason: finish}, err
}

func (gateway *Gateway) fallback(request Request) Response {
	if request.Task == "classify" {
		result := classify.From(nil, request.Input, "")
		return Response{Text: result.Slug, Provider: "keyword-rules", Model: result.Model}
	}
	return Response{Provider: "none", Model: "none"}
}

func intPointer(value int) *int { return &value }

func blockedStatus(status string, checkedAt *time.Time, now time.Time, cooldown time.Duration) bool {
	return (status == "auth_expired" || status == "quota_exhausted") && checkedAt != nil && now.Sub(*checkedAt) < cooldown
}

func (gateway *Gateway) providerBlockError(ctx context.Context) error {
	var status, detail string
	var checkedAt *time.Time
	err := gateway.pool.QueryRow(ctx, `SELECT status, COALESCE(status_detail, ''), checked_at FROM llm_providers WHERE id = 'codex_cli'`).Scan(&status, &detail, &checkedAt)
	if err != nil {
		return fmt.Errorf("load provider cooldown: %w", err)
	}
	if !blockedStatus(status, checkedAt, time.Now(), gateway.cooldown) {
		return nil
	}
	kind := codex.ErrQuota
	if status == "auth_expired" {
		kind = codex.ErrAuth
	}
	return fmt.Errorf("%w: %w: %s", ErrProviderBlocked, kind, detail)
}

func (gateway *Gateway) ProviderBlocked(ctx context.Context) (bool, error) {
	err := gateway.providerBlockError(ctx)
	if errors.Is(err, codex.ErrAuth) || errors.Is(err, codex.ErrQuota) {
		return true, nil
	}
	return false, err
}

func (gateway *Gateway) recordCallStatus(ctx context.Context, callErr error, started time.Time) {
	status, detail := "operational", "Codex completion succeeded."
	if callErr != nil {
		detail = callErr.Error()
		switch {
		case errors.Is(callErr, codex.ErrAuth):
			status = "auth_expired"
		case errors.Is(callErr, codex.ErrQuota):
			status = "quota_exhausted"
		default:
			return
		}
	}
	// An older in-flight success must not erase a newer quota/auth failure.
	_, _ = gateway.pool.Exec(ctx, `UPDATE llm_providers SET status=$1,status_detail=$2,checked_at=clock_timestamp()
		WHERE id='codex_cli' AND ($1 <> 'operational' OR status NOT IN ('auth_expired','quota_exhausted')
		  OR checked_at IS NULL OR checked_at <= $3)`, status, detail, started)
}

func (gateway *Gateway) startCall(ctx context.Context, request Request, profile providerProfile) int64 {
	if request.PipelineStepID != "" {
		_, _ = gateway.pool.Exec(ctx, `
			UPDATE llm_calls
			SET outcome = 'fallback', error_detail = 'Superseded by a retried provider request.',
			    latency_ms = GREATEST(0, (extract(epoch FROM clock_timestamp() - created_at) * 1000)::integer),
			    completed_at = clock_timestamp()
			WHERE pipeline_step_id = $1 AND outcome = 'running'
		`, request.PipelineStepID)
	}
	var id int64
	_ = gateway.pool.QueryRow(ctx, `
		INSERT INTO llm_calls (
		  task, provider_id, model, outcome, article_id, pipeline_run_id, pipeline_step_id, streamed
		)
		VALUES ($1, $2, $3, 'running', NULLIF($4, '')::uuid, NULLIF($5, '')::uuid, NULLIF($6, '')::uuid, false)
		RETURNING id
	`, request.Task, profile.id, profile.model, request.ArticleID, request.PipelineRunID, request.PipelineStepID).Scan(&id)
	return id
}

func (gateway *Gateway) finishCall(ctx context.Context, callID int64, outcome string, latencyMS int64, result providerResult, callErr error) {
	if callID == 0 {
		return
	}
	errorDetail := ""
	if callErr != nil {
		errorDetail = callErr.Error()
	}
	_, _ = gateway.pool.Exec(ctx, `
		UPDATE llm_calls
		SET latency_ms = $2, outcome = $3, input_tokens = $4, output_tokens = $5,
		    first_token_ms = COALESCE(first_token_ms, $6), response_text = NULLIF($7, ''),
		    finish_reason = NULLIF($8, ''), error_detail = NULLIF($9, ''), completed_at = clock_timestamp()
		WHERE id = $1
	`, callID, latencyMS, outcome, result.InputTokens, result.OutputTokens, result.FirstTokenMS,
		result.Text, result.FinishReason, errorDetail)
}

func (gateway *Gateway) logPipeline(ctx context.Context, request Request, level, event, message string, details map[string]any, createdAt time.Time) {
	if request.PipelineRunID == "" {
		return
	}
	payload, err := json.Marshal(details)
	if err != nil {
		return
	}
	_, _ = gateway.pool.Exec(ctx, `
		INSERT INTO article_pipeline_logs (run_id, step_id, level, event, message, details, created_at)
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, $6, $7)
	`, request.PipelineRunID, request.PipelineStepID, level, event, message, payload, createdAt)
}

type Provider struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Enabled       bool      `json:"enabled"`
	Available     bool      `json:"available"`
	Status        string    `json:"status"`
	StatusDetail  string    `json:"status_detail"`
	CheckedAt     time.Time `json:"checked_at"`
	Installed     bool      `json:"installed"`
	Authenticated bool      `json:"authenticated"`
	Ready         bool      `json:"ready"`
	Version       string    `json:"version"`
	AuthMethod    string    `json:"auth_method"`
}

func (gateway *Gateway) ProviderStatus(ctx context.Context) (Provider, error) {
	var item Provider
	var storedStatus, storedDetail string
	var storedCheckedAt *time.Time
	err := gateway.pool.QueryRow(ctx, `
		SELECT id, enabled, status, COALESCE(status_detail, ''), checked_at
		FROM llm_providers WHERE id = 'codex_cli' AND kind = 'codex_cli'
	`).Scan(&item.ID, &item.Enabled, &storedStatus, &storedDetail, &storedCheckedAt)
	if err != nil {
		return Provider{}, fmt.Errorf("load Codex provider: %w", err)
	}
	probe := gateway.client.Probe(ctx)
	item.Name, item.CheckedAt = "Codex CLI", probe.CheckedAt
	item.Installed, item.Authenticated, item.Version, item.AuthMethod = probe.Installed, probe.Authenticated, probe.Version, probe.AuthMethod
	item.Status, item.StatusDetail = "unavailable", probe.Detail
	if probe.Ready {
		item.Status = "ready"
	}
	if !item.Enabled {
		item.Status, item.StatusDetail = "disabled", "Provider is disabled."
	}
	if item.Enabled && blockedStatus(storedStatus, storedCheckedAt, time.Now(), gateway.cooldown) {
		item.Status, item.StatusDetail, item.CheckedAt = storedStatus, storedDetail, *storedCheckedAt
	}
	item.Ready = item.Enabled && probe.Ready && item.Status == "ready"
	item.Available = item.Ready
	// A local login probe cannot establish that quota is restored. It must never
	// overwrite a call failure or renew its cooldown timestamp.
	return item, nil
}

type Model struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	CompatibleTasks  []string `json:"compatible_tasks"`
	ReasoningEfforts []string `json:"reasoning_efforts"`
}

func (gateway *Gateway) ListModels(context.Context) ([]Model, error) {
	models := make([]Model, 0)
	for _, id := range codex.Models() {
		models = append(models, Model{ID: id, Name: id, CompatibleTasks: append([]string{}, orderedTasks...), ReasoningEfforts: []string{"low", "medium", "high"}})
	}
	return models, nil
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

type TaskProfile struct {
	Task            string `json:"task"`
	Name            string `json:"name"`
	Purpose         string `json:"purpose"`
	Provider        string `json:"provider_id"`
	Model           string `json:"model"`
	Timeout         int    `json:"timeout_seconds"`
	Enabled         bool   `json:"enabled"`
	ReasoningEffort string `json:"reasoning_effort"`
}

var taskDefinitions = map[string]struct{ name, purpose string }{
	"article_analysis": {
		name:    "Article analysis",
		purpose: "Cleans, summarizes, categorizes and evaluates narration for one article in a single call.",
	},
	"content_cleaning": {
		name:    "Content cleaning",
		purpose: "Turns captured publisher pages into faithful, readable Markdown.",
	},
	"article_summary": {
		name:    "Article summary",
		purpose: "Creates factual summaries and key points from cleaned articles.",
	},
	"classify": {
		name:    "Classification",
		purpose: "Assigns a newsroom category when keyword confidence is low.",
	},
	"narration_framing": {
		name:    "Narration analysis",
		purpose: "Identifies political-economic framing and supporting evidence.",
	},
	"event_synthesis": {
		name:    "Event synthesis",
		purpose: "Combines multiple reports about one event while preserving source differences.",
	},
	"admin_article_analysis": {
		name:    "Administrative analysis",
		purpose: "Produces detailed structured analysis for editorial review.",
	},
	"watch_tower_retrieval": {
		name:    "Watch Tower retrieval",
		purpose: "Builds bilingual search plans for newsroom questions.",
	},
	"watch_tower_answer": {
		name:    "Watch Tower answers",
		purpose: "Synthesizes cited answers from the retrieved newsroom evidence.",
	},
	"social_post_editorial": {
		name:    "Social post editorial",
		purpose: "Selects public-interest stories and writes original Sinhala Facebook Page posts.",
	},
	"newsletter_editorial": {
		name:    "Morning newsletter",
		purpose: "Prepares the last-24-hours email edition from approved published stories.",
	},
}

var orderedTasks = []string{
	"article_analysis", "content_cleaning", "article_summary", "classify", "event_synthesis",
	"narration_framing", "admin_article_analysis", "watch_tower_retrieval",
	"watch_tower_answer", "newsletter_editorial", "social_post_editorial",
}

func (gateway *Gateway) ListProfiles(ctx context.Context) ([]TaskProfile, error) {
	rows, err := gateway.pool.Query(ctx, `
		SELECT task, provider_id, model, timeout_seconds, enabled, COALESCE(reasoning_effort, '')
		FROM llm_task_profiles
		WHERE task IN (
		  'article_analysis', 'content_cleaning', 'article_summary', 'classify', 'event_synthesis',
		  'narration_framing', 'admin_article_analysis', 'watch_tower_retrieval',
		  'watch_tower_answer', 'newsletter_editorial', 'social_post_editorial'
		)
		  AND provider_id = 'codex_cli'
		ORDER BY task
	`)
	if err != nil {
		return nil, fmt.Errorf("list LLM profiles: %w", err)
	}
	defer rows.Close()
	items := make([]TaskProfile, 0, len(taskDefinitions))
	for rows.Next() {
		var item TaskProfile
		if err := rows.Scan(&item.Task, &item.Provider, &item.Model, &item.Timeout, &item.Enabled, &item.ReasoningEffort); err != nil {
			return nil, fmt.Errorf("scan LLM profile: %w", err)
		}
		definition := taskDefinitions[item.Task]
		item.Name, item.Purpose = definition.name, definition.purpose
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate LLM profiles: %w", err)
	}
	return items, nil
}

func (gateway *Gateway) UpdateProfile(ctx context.Context, task, providerID, modelID, effort string) error {
	if err := validateProfile(task, providerID, modelID, effort); err != nil {
		return err
	}
	tag, err := gateway.pool.Exec(ctx, `
		UPDATE llm_task_profiles
		SET provider_id = $2, model = $3, reasoning_effort = NULLIF($4, ''), enabled = true
		WHERE task = $1
	`, task, providerID, modelID, effort)
	if err != nil {
		return fmt.Errorf("update LLM profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("task profile %q was not found", task)
	}
	return nil
}

func validateProfile(task, providerID, modelID, effort string) error {
	if _, ok := taskDefinitions[task]; !ok {
		return fmt.Errorf("unsupported task %q", task)
	}
	if providerID != "codex_cli" {
		return fmt.Errorf("unsupported provider %q", providerID)
	}
	if !codex.ValidModel(modelID) {
		return fmt.Errorf("model %q is unavailable", modelID)
	}
	if !codex.ValidReasoningEffort(effort) {
		return fmt.Errorf("invalid reasoning effort %q", effort)
	}
	return nil
}
