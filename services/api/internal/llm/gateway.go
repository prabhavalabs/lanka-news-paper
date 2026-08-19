package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/classify"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/pagination"
)

type Request struct {
	Task             string
	System           string
	Input            string
	JSONSchema       map[string]any
	DisableReasoning bool
	MaxTokens        int
	ArticleID        string
	PipelineRunID    string
	PipelineStepID   string
}

type Response struct {
	Text     string
	Provider string
	Model    string
}

type providerProfile struct {
	id, kind, baseURL, keyRef, model string
	timeout                          int
}

type providerResult struct {
	Text         string
	InputTokens  *int
	OutputTokens *int
	FirstTokenMS *int
	FinishReason string
}

type Gateway struct {
	pool   *pgxpool.Pool
	client *http.Client
}

func NewGateway(pool *pgxpool.Pool) *Gateway {
	return &Gateway{pool: pool, client: &http.Client{}}
}

func (gateway *Gateway) Complete(ctx context.Context, request Request) (Response, error) {
	rows, err := gateway.pool.Query(ctx, `
		SELECT p.id, p.kind, COALESCE(p.base_url, ''), COALESCE(p.api_key_ref, ''), t.model, t.timeout_seconds
		FROM llm_task_profiles t
		JOIN llm_providers p ON p.id = t.provider_id
		WHERE t.task = $1 AND t.enabled AND p.enabled
		ORDER BY t.priority
	`, request.Task)
	if err != nil {
		return gateway.fallback(request), nil
	}
	profiles := make([]providerProfile, 0)
	for rows.Next() {
		var profile providerProfile
		if err := rows.Scan(&profile.id, &profile.kind, &profile.baseURL, &profile.keyRef, &profile.model, &profile.timeout); err != nil {
			continue
		}
		profiles = append(profiles, profile)
	}
	rowsError := rows.Err()
	rows.Close()
	if rowsError != nil && rowsError != pgx.ErrNoRows {
		return gateway.fallback(request), nil
	}

	for _, profile := range profiles {
		started := time.Now()
		callID := gateway.startCall(ctx, request, profile)
		gateway.logPipeline(ctx, request, "info", "provider_request_started", "Provider request started", map[string]any{
			"call_id": callID, "provider": profile.id, "model": profile.model,
			"task": request.Task, "timeout_seconds": profile.timeout, "streamed": true,
		}, started)

		callContext, cancelCall := context.WithTimeout(ctx, time.Duration(profile.timeout)*time.Second)
		result, callErr := gateway.callProvider(callContext, profile.kind, profile.baseURL, profile.keyRef, profile.model, request, func(firstTokenMS int) {
			gateway.recordFirstToken(ctx, callID, firstTokenMS)
			gateway.logPipeline(ctx, request, "info", "provider_first_token", "First token received", map[string]any{
				"call_id": callID, "provider": profile.id, "model": profile.model, "first_token_ms": firstTokenMS,
			}, time.Now())
		})
		cancelCall()
		latencyMS := time.Since(started).Milliseconds()
		recordContext, cancelRecord := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		if callErr != nil {
			gateway.finishCall(recordContext, callID, "fallback", latencyMS, result, callErr)
			gateway.logPipeline(recordContext, request, "error", "provider_request_failed", "Provider request failed", map[string]any{
				"call_id": callID, "provider": profile.id, "model": profile.model,
				"latency_ms": latencyMS, "error": callErr.Error(),
			}, time.Now())
			cancelRecord()
			continue
		}
		gateway.finishCall(recordContext, callID, "ok", latencyMS, result, nil)
		gateway.logPipeline(recordContext, request, "info", "provider_response_completed", "Structured response completed", map[string]any{
			"call_id": callID, "provider": profile.id, "model": profile.model,
			"latency_ms": latencyMS, "first_token_ms": result.FirstTokenMS,
			"input_tokens": result.InputTokens, "output_tokens": result.OutputTokens,
			"finish_reason": result.FinishReason,
		}, time.Now())
		cancelRecord()
		return Response{Text: result.Text, Provider: profile.id, Model: profile.model}, nil
	}
	return gateway.fallback(request), nil
}

func (gateway *Gateway) fallback(request Request) Response {
	if request.Task == "classify" {
		result := classify.From(nil, request.Input, "")
		return Response{Text: result.Slug, Provider: "keyword-rules", Model: result.Model}
	}
	return Response{Text: "", Provider: "none", Model: "none"}
}

func (gateway *Gateway) callProvider(ctx context.Context, kind, baseURL, keyRef, model string, request Request, onFirstToken func(int)) (providerResult, error) {
	if kind != "openai_api" && kind != "openai_compatible" {
		return providerResult{}, fmt.Errorf("codex_cli is not configured")
	}
	if baseURL == "" {
		return providerResult{}, fmt.Errorf("missing base_url")
	}
	apiKey := ""
	if keyRef != "" {
		apiKey = os.Getenv(keyRef)
	}
	if kind == "openai_api" && keyRef == "" {
		return providerResult{}, fmt.Errorf("missing api key reference")
	}
	if keyRef != "" && apiKey == "" {
		return providerResult{}, fmt.Errorf("missing secret %s", keyRef)
	}
	payloadValue := map[string]any{
		"model":          model,
		"stream":         true,
		"stream_options": map[string]any{"include_usage": true},
		"temperature":    0,
		"messages": []map[string]string{
			{"role": "system", "content": request.System},
			{"role": "user", "content": request.Input},
		},
	}
	isOpenRouter := strings.HasPrefix(strings.TrimRight(baseURL, "/"), "https://openrouter.ai/api/v1")
	if request.JSONSchema != nil {
		payloadValue["response_format"] = map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   request.Task,
				"strict": true,
				"schema": request.JSONSchema,
			},
		}
		if isOpenRouter {
			payloadValue["provider"] = map[string]any{"require_parameters": true}
		}
	}
	if request.DisableReasoning {
		if isOpenRouter {
			payloadValue["reasoning"] = map[string]any{"effort": "none"}
		} else {
			payloadValue["reasoning_effort"] = "none"
		}
	}
	if request.MaxTokens > 0 {
		payloadValue["max_tokens"] = request.MaxTokens
	}
	payload, err := json.Marshal(payloadValue)
	if err != nil {
		return providerResult{}, fmt.Errorf("encode provider request: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return providerResult{}, err
	}
	if apiKey != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	started := time.Now()
	response, err := gateway.client.Do(httpRequest)
	if err != nil {
		return providerResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			return providerResult{}, fmt.Errorf("provider status %d", response.StatusCode)
		}
		return providerResult{}, fmt.Errorf("provider status %d: %s", response.StatusCode, detail)
	}

	var result providerResult
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return result, fmt.Errorf("decode provider stream: %w", err)
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			return result, fmt.Errorf("provider stream: %s", chunk.Error.Message)
		}
		if chunk.Usage != nil {
			result.InputTokens = intPointer(chunk.Usage.PromptTokens)
			result.OutputTokens = intPointer(chunk.Usage.CompletionTokens)
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				if result.FirstTokenMS == nil {
					elapsed := int(time.Since(started).Milliseconds())
					result.FirstTokenMS = &elapsed
					if onFirstToken != nil {
						onFirstToken(elapsed)
					}
				}
				result.Text += choice.Delta.Content
			}
			if choice.FinishReason != nil {
				result.FinishReason = *choice.FinishReason
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("read provider stream: %w", err)
	}
	if result.Text == "" {
		return result, fmt.Errorf("empty completion")
	}
	return result, nil
}

func intPointer(value int) *int {
	return &value
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
		VALUES ($1, $2, $3, 'running', NULLIF($4, '')::uuid, NULLIF($5, '')::uuid, NULLIF($6, '')::uuid, true)
		RETURNING id
	`, request.Task, profile.id, profile.model, request.ArticleID, request.PipelineRunID, request.PipelineStepID).Scan(&id)
	return id
}

func (gateway *Gateway) recordFirstToken(ctx context.Context, callID int64, firstTokenMS int) {
	if callID == 0 {
		return
	}
	_, _ = gateway.pool.Exec(ctx, `UPDATE llm_calls SET first_token_ms = $2 WHERE id = $1`, callID, firstTokenMS)
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
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	BaseURL string `json:"base_url"`
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
	KeySet  bool   `json:"key_set"`
}

func (gateway *Gateway) ListProviders(ctx context.Context, params pagination.Params, state string) ([]Provider, int, error) {
	var total int
	err := gateway.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM llm_providers
		WHERE ($1 = '' OR id ILIKE '%' || $1 || '%' OR kind ILIKE '%' || $1 || '%'
		       OR COALESCE(base_url, '') ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR ($2 = 'enabled' AND enabled) OR ($2 = 'disabled' AND NOT enabled))
	`, params.Search, state).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count LLM providers: %w", err)
	}

	rows, err := gateway.pool.Query(ctx, `
		SELECT id, kind, COALESCE(base_url, ''), enabled, status,
		       kind = 'openai_compatible' OR (api_key_ref IS NOT NULL AND api_key_ref <> '')
		FROM llm_providers
		WHERE ($1 = '' OR id ILIKE '%' || $1 || '%' OR kind ILIKE '%' || $1 || '%'
		       OR COALESCE(base_url, '') ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR ($2 = 'enabled' AND enabled) OR ($2 = 'disabled' AND NOT enabled))
		ORDER BY id
		LIMIT $3 OFFSET $4
	`, params.Search, state, params.Limit(), params.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list LLM providers: %w", err)
	}
	defer rows.Close()
	items := make([]Provider, 0)
	for rows.Next() {
		var item Provider
		if err := rows.Scan(&item.ID, &item.Kind, &item.BaseURL, &item.Enabled, &item.Status, &item.KeySet); err != nil {
			return nil, 0, fmt.Errorf("scan LLM provider: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate LLM providers: %w", err)
	}
	return items, total, nil
}

func (gateway *Gateway) UpsertProvider(ctx context.Context, item Provider, keyRef string) error {
	_, err := gateway.pool.Exec(ctx, `
		INSERT INTO llm_providers (id, kind, base_url, api_key_ref, enabled, status)
		VALUES ($1, $2, $3, $4, $5, 'unknown')
		ON CONFLICT (id) DO UPDATE SET kind = EXCLUDED.kind, base_url = EXCLUDED.base_url,
		  api_key_ref = EXCLUDED.api_key_ref, enabled = EXCLUDED.enabled
	`, item.ID, item.Kind, item.BaseURL, keyRef, item.Enabled)
	return err
}

type TaskProfile struct {
	Task     string `json:"task"`
	Priority int    `json:"priority"`
	Provider string `json:"provider_id"`
	Model    string `json:"model"`
	Timeout  int    `json:"timeout_seconds"`
	Enabled  bool   `json:"enabled"`
}

func (gateway *Gateway) ListProfiles(ctx context.Context, params pagination.Params, state string) ([]TaskProfile, int, error) {
	var total int
	err := gateway.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM llm_task_profiles
		WHERE ($1 = '' OR task ILIKE '%' || $1 || '%' OR provider_id ILIKE '%' || $1 || '%'
		       OR model ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR ($2 = 'enabled' AND enabled) OR ($2 = 'disabled' AND NOT enabled))
	`, params.Search, state).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count LLM profiles: %w", err)
	}

	rows, err := gateway.pool.Query(ctx, `
		SELECT task, priority, provider_id, model, timeout_seconds, enabled
		FROM llm_task_profiles
		WHERE ($1 = '' OR task ILIKE '%' || $1 || '%' OR provider_id ILIKE '%' || $1 || '%'
		       OR model ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR ($2 = 'enabled' AND enabled) OR ($2 = 'disabled' AND NOT enabled))
		ORDER BY task, priority
		LIMIT $3 OFFSET $4
	`, params.Search, state, params.Limit(), params.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list LLM profiles: %w", err)
	}
	defer rows.Close()
	items := make([]TaskProfile, 0)
	for rows.Next() {
		var item TaskProfile
		if err := rows.Scan(&item.Task, &item.Priority, &item.Provider, &item.Model, &item.Timeout, &item.Enabled); err != nil {
			return nil, 0, fmt.Errorf("scan LLM profile: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate LLM profiles: %w", err)
	}
	return items, total, nil
}

func (gateway *Gateway) UpsertProfile(ctx context.Context, item TaskProfile) error {
	if item.Timeout <= 0 {
		item.Timeout = 60
	}
	_, err := gateway.pool.Exec(ctx, `
		INSERT INTO llm_task_profiles (task, priority, provider_id, model, timeout_seconds, enabled)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (task, priority) DO UPDATE SET provider_id = EXCLUDED.provider_id, model = EXCLUDED.model,
		  timeout_seconds = EXCLUDED.timeout_seconds, enabled = EXCLUDED.enabled
	`, item.Task, item.Priority, item.Provider, item.Model, item.Timeout, item.Enabled)
	return err
}
