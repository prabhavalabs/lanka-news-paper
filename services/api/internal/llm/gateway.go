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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/classify"
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
	var profile providerProfile
	err := gateway.pool.QueryRow(ctx, `
		SELECT p.id, p.kind, COALESCE(p.base_url, ''), COALESCE(p.api_key_ref, ''), t.model, t.timeout_seconds
		FROM llm_task_profiles t
		JOIN llm_providers p ON p.id = t.provider_id
		WHERE t.task = $1 AND t.enabled AND p.enabled
	`, request.Task).Scan(&profile.id, &profile.kind, &profile.baseURL, &profile.keyRef, &profile.model, &profile.timeout)
	if err != nil && err != pgx.ErrNoRows {
		return gateway.fallback(request), nil
	}
	if err == pgx.ErrNoRows {
		return gateway.fallback(request), nil
	}
	return gateway.completeWithProfile(ctx, request, profile, true)
}

// CompleteWithModel performs one strict, explicitly configured OpenRouter call.
// Administrative backfills use this path so a failed provider request remains
// visible and retryable instead of silently falling back to keyword rules.
func (gateway *Gateway) CompleteWithModel(ctx context.Context, request Request, providerID, model string) (Response, error) {
	providerID = strings.TrimSpace(providerID)
	model = strings.TrimSpace(model)
	if providerID != "openrouter" {
		return Response{}, fmt.Errorf("unsupported explicit provider %q", providerID)
	}
	if model == "" {
		return Response{}, fmt.Errorf("model is required")
	}
	var profile providerProfile
	err := gateway.pool.QueryRow(ctx, `
		SELECT id, kind, COALESCE(base_url, ''), COALESCE(api_key_ref, ''), $2::text, 600
		FROM llm_providers
		WHERE id = $1 AND enabled
	`, providerID, model).Scan(
		&profile.id, &profile.kind, &profile.baseURL, &profile.keyRef, &profile.model, &profile.timeout,
	)
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
	defer cancelRecord()
	if callErr != nil {
		outcome := "failed"
		if allowFallback {
			outcome = "fallback"
		}
		gateway.finishCall(recordContext, callID, outcome, latencyMS, result, callErr)
		gateway.logPipeline(recordContext, request, "error", "provider_request_failed", "Provider request failed", map[string]any{
			"call_id": callID, "provider": profile.id, "model": profile.model,
			"latency_ms": latencyMS, "error": callErr.Error(),
		}, time.Now())
		if allowFallback {
			return gateway.fallback(request), nil
		}
		return Response{}, callErr
	}
	gateway.finishCall(recordContext, callID, "ok", latencyMS, result, nil)
	gateway.logPipeline(recordContext, request, "info", "provider_response_completed", "Structured response completed", map[string]any{
		"call_id": callID, "provider": profile.id, "model": profile.model,
		"latency_ms": latencyMS, "first_token_ms": result.FirstTokenMS,
		"input_tokens": result.InputTokens, "output_tokens": result.OutputTokens,
		"finish_reason": result.FinishReason,
	}, time.Now())
	return Response{Text: result.Text, Provider: profile.id, Model: profile.model}, nil
}

func (gateway *Gateway) fallback(request Request) Response {
	if request.Task == "classify" {
		result := classify.From(nil, request.Input, "")
		return Response{Text: result.Slug, Provider: "keyword-rules", Model: result.Model}
	}
	return Response{Text: "", Provider: "none", Model: "none"}
}

func (gateway *Gateway) callProvider(ctx context.Context, kind, baseURL, keyRef, model string, request Request, onFirstToken func(int)) (providerResult, error) {
	if kind != "openai_api" {
		return providerResult{}, fmt.Errorf("unsupported provider kind %q", kind)
	}
	if baseURL == "" {
		return providerResult{}, fmt.Errorf("missing base_url")
	}
	apiKey := ""
	if keyRef != "" {
		apiKey = os.Getenv(keyRef)
	}
	if keyRef == "" {
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
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	BaseURL           string     `json:"base_url"`
	Enabled           bool       `json:"enabled"`
	Available         bool       `json:"available"`
	Status            string     `json:"status"`
	StatusDetail      string     `json:"status_detail"`
	KeySet            bool       `json:"key_set"`
	LatencyMS         int64      `json:"latency_ms"`
	CheckedAt         time.Time  `json:"checked_at"`
	FreeTier          bool       `json:"free_tier"`
	LimitUSD          *float64   `json:"limit_usd"`
	LimitRemainingUSD *float64   `json:"limit_remaining_usd"`
	ExpiresAt         *time.Time `json:"expires_at"`
}

type providerConfig struct {
	id, baseURL, keyRef string
	enabled             bool
}

type openRouterKeyStatus struct {
	FreeTier          bool       `json:"is_free_tier"`
	LimitUSD          *float64   `json:"limit"`
	LimitRemainingUSD *float64   `json:"limit_remaining"`
	ExpiresAt         *time.Time `json:"expires_at"`
	LatencyMS         int64      `json:"-"`
}

func (gateway *Gateway) ProviderStatus(ctx context.Context) (Provider, error) {
	config, err := gateway.openRouterConfig(ctx)
	if err != nil {
		return Provider{}, err
	}
	item := Provider{
		ID: config.id, Name: "OpenRouter", BaseURL: config.baseURL, Enabled: config.enabled,
		Status: "disabled", CheckedAt: time.Now().UTC(),
	}
	key := os.Getenv(config.keyRef)
	item.KeySet = key != ""
	if !config.enabled {
		item.StatusDetail = "Provider is disabled."
		return item, nil
	}
	if key == "" {
		item.Status = "misconfigured"
		item.StatusDetail = "OPENROUTER_API_KEY is not configured."
		gateway.recordProviderStatus(ctx, item)
		return item, nil
	}

	checkContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	status, checkErr := gateway.checkOpenRouter(checkContext, config.baseURL, key)
	item.LatencyMS = status.LatencyMS
	item.FreeTier = status.FreeTier
	item.LimitUSD = status.LimitUSD
	item.LimitRemainingUSD = status.LimitRemainingUSD
	item.ExpiresAt = status.ExpiresAt
	if checkErr != nil {
		item.Status = "unavailable"
		item.StatusDetail = checkErr.Error()
		gateway.recordProviderStatus(ctx, item)
		return item, nil
	}
	item.Available = true
	item.Status = "operational"
	item.StatusDetail = "OpenRouter accepted the configured API key."
	gateway.recordProviderStatus(ctx, item)
	return item, nil
}

func (gateway *Gateway) checkOpenRouter(ctx context.Context, baseURL, key string) (openRouterKeyStatus, error) {
	var status openRouterKeyStatus
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/key", nil)
	if err != nil {
		return status, err
	}
	request.Header.Set("Authorization", "Bearer "+key)
	started := time.Now()
	response, err := gateway.client.Do(request)
	status.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		return status, fmt.Errorf("OpenRouter health check failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return status, fmt.Errorf("OpenRouter health check returned HTTP %d", response.StatusCode)
	}
	var payload struct {
		Data openRouterKeyStatus `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 256*1024)).Decode(&payload); err != nil {
		return status, fmt.Errorf("decode OpenRouter health response: %w", err)
	}
	payload.Data.LatencyMS = status.LatencyMS
	return payload.Data, nil
}

func (gateway *Gateway) recordProviderStatus(ctx context.Context, item Provider) {
	_, _ = gateway.pool.Exec(ctx, `
		UPDATE llm_providers
		SET status = $2, status_detail = NULLIF($3, ''), checked_at = $4
		WHERE id = $1
	`, item.ID, item.Status, item.StatusDetail, item.CheckedAt)
}

func (gateway *Gateway) openRouterConfig(ctx context.Context) (providerConfig, error) {
	var config providerConfig
	err := gateway.pool.QueryRow(ctx, `
		SELECT id, COALESCE(base_url, ''), COALESCE(api_key_ref, ''), enabled
		FROM llm_providers
		WHERE id = 'openrouter' AND kind = 'openai_api'
	`).Scan(&config.id, &config.baseURL, &config.keyRef, &config.enabled)
	if err != nil {
		return providerConfig{}, fmt.Errorf("load OpenRouter configuration: %w", err)
	}
	return config, nil
}

type Model struct {
	ID                    string   `json:"id"`
	Name                  string   `json:"name"`
	ContextLength         int      `json:"context_length"`
	InputPricePerMillion  float64  `json:"input_price_per_million"`
	OutputPricePerMillion float64  `json:"output_price_per_million"`
	InputModalities       []string `json:"input_modalities"`
	OutputModalities      []string `json:"output_modalities"`
	SupportedParameters   []string `json:"supported_parameters"`
	CompatibleTasks       []string `json:"compatible_tasks"`
}

func (gateway *Gateway) ListModels(ctx context.Context) ([]Model, error) {
	config, err := gateway.openRouterConfig(ctx)
	if err != nil {
		return nil, err
	}
	key := os.Getenv(config.keyRef)
	if !config.enabled || key == "" {
		return nil, fmt.Errorf("OpenRouter is not configured")
	}
	requestContext, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, strings.TrimRight(config.baseURL, "/")+"/models?output_modalities=text", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+key)
	response, err := gateway.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch OpenRouter models: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenRouter models returned HTTP %d", response.StatusCode)
	}
	return decodeOpenRouterModels(io.LimitReader(response.Body, 8*1024*1024))
}

func decodeOpenRouterModels(reader io.Reader) ([]Model, error) {
	var payload struct {
		Data []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			ContextLength int    `json:"context_length"`
			Pricing       struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
			Architecture struct {
				InputModalities  []string `json:"input_modalities"`
				OutputModalities []string `json:"output_modalities"`
			} `json:"architecture"`
			SupportedParameters []string `json:"supported_parameters"`
		} `json:"data"`
	}
	if err := json.NewDecoder(reader).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode OpenRouter models: %w", err)
	}
	models := make([]Model, 0, len(payload.Data))
	for _, value := range payload.Data {
		inputPrice, err := perMillionPrice(value.Pricing.Prompt)
		if err != nil {
			return nil, fmt.Errorf("decode input price for %s: %w", value.ID, err)
		}
		outputPrice, err := perMillionPrice(value.Pricing.Completion)
		if err != nil {
			return nil, fmt.Errorf("decode output price for %s: %w", value.ID, err)
		}
		model := Model{
			ID: value.ID, Name: value.Name, ContextLength: value.ContextLength,
			InputPricePerMillion: inputPrice, OutputPricePerMillion: outputPrice,
			InputModalities: append([]string{}, value.Architecture.InputModalities...), OutputModalities: append([]string{}, value.Architecture.OutputModalities...),
			SupportedParameters: append([]string{}, value.SupportedParameters...), CompatibleTasks: make([]string, 0, 2),
		}
		for _, task := range []string{"classify", "narration_framing"} {
			if modelSupportsTask(model, task) {
				model.CompatibleTasks = append(model.CompatibleTasks, task)
			}
		}
		models = append(models, model)
	}
	sort.Slice(models, func(left, right int) bool { return models[left].Name < models[right].Name })
	return models, nil
}

func perMillionPrice(value string) (float64, error) {
	if value == "" {
		return 0, nil
	}
	price, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	return price * 1_000_000, nil
}

func modelSupportsTask(model Model, task string) bool {
	if !contains(model.OutputModalities, "text") || !contains(model.SupportedParameters, "max_tokens") {
		return false
	}
	if task == "narration_framing" {
		return contains(model.SupportedParameters, "structured_outputs") || contains(model.SupportedParameters, "response_format")
	}
	return task == "classify"
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
	Task     string `json:"task"`
	Name     string `json:"name"`
	Purpose  string `json:"purpose"`
	Provider string `json:"provider_id"`
	Model    string `json:"model"`
	Timeout  int    `json:"timeout_seconds"`
	Enabled  bool   `json:"enabled"`
}

var taskDefinitions = map[string]struct{ name, purpose string }{
	"classify": {
		name:    "Classification",
		purpose: "Assigns a newsroom category when keyword confidence is low.",
	},
	"narration_framing": {
		name:    "Narration analysis",
		purpose: "Identifies political-economic framing and supporting evidence.",
	},
}

func (gateway *Gateway) ListProfiles(ctx context.Context) ([]TaskProfile, error) {
	rows, err := gateway.pool.Query(ctx, `
		SELECT task, provider_id, model, timeout_seconds, enabled
		FROM llm_task_profiles
		WHERE task IN ('classify', 'narration_framing') AND provider_id = 'openrouter'
		ORDER BY CASE task WHEN 'classify' THEN 1 ELSE 2 END
	`)
	if err != nil {
		return nil, fmt.Errorf("list LLM profiles: %w", err)
	}
	defer rows.Close()
	items := make([]TaskProfile, 0, len(taskDefinitions))
	for rows.Next() {
		var item TaskProfile
		if err := rows.Scan(&item.Task, &item.Provider, &item.Model, &item.Timeout, &item.Enabled); err != nil {
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

func (gateway *Gateway) UpdateProfile(ctx context.Context, task, modelID string) error {
	if _, ok := taskDefinitions[task]; !ok {
		return fmt.Errorf("unsupported task %q", task)
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return fmt.Errorf("model is required")
	}
	models, err := gateway.ListModels(ctx)
	if err != nil {
		return err
	}
	valid := false
	for _, model := range models {
		if model.ID == modelID && contains(model.CompatibleTasks, task) {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("model %q is unavailable or incompatible with %s", modelID, task)
	}
	tag, err := gateway.pool.Exec(ctx, `
		UPDATE llm_task_profiles
		SET provider_id = 'openrouter', model = $2, enabled = true
		WHERE task = $1 AND provider_id = 'openrouter'
	`, task, modelID)
	if err != nil {
		return fmt.Errorf("update LLM profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("task profile %q was not found", task)
	}
	return nil
}
