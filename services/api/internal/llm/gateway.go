package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	Task   string
	System string
	Input  string
}

type Response struct {
	Text     string
	Provider string
	Model    string
}

type Gateway struct {
	pool   *pgxpool.Pool
	client *http.Client
}

func NewGateway(pool *pgxpool.Pool) *Gateway {
	return &Gateway{pool: pool, client: &http.Client{Timeout: 45 * time.Second}}
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
	defer rows.Close()

	for rows.Next() {
		var providerID, kind, baseURL, keyRef, model string
		var timeout int
		if err := rows.Scan(&providerID, &kind, &baseURL, &keyRef, &model, &timeout); err != nil {
			continue
		}
		started := time.Now()
		text, callErr := gateway.callProvider(ctx, kind, baseURL, keyRef, model, request)
		outcome := "ok"
		if callErr != nil {
			outcome = "fallback"
			_, _ = gateway.pool.Exec(ctx, `
				INSERT INTO llm_calls (task, provider_id, model, latency_ms, outcome)
				VALUES ($1, $2, $3, $4, $5)
			`, request.Task, providerID, model, time.Since(started).Milliseconds(), outcome)
			continue
		}
		_, _ = gateway.pool.Exec(ctx, `
			INSERT INTO llm_calls (task, provider_id, model, latency_ms, outcome)
			VALUES ($1, $2, $3, $4, 'ok')
		`, request.Task, providerID, model, time.Since(started).Milliseconds())
		return Response{Text: text, Provider: providerID, Model: model}, nil
	}
	if err := rows.Err(); err != nil && err != pgx.ErrNoRows {
		return gateway.fallback(request), nil
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

func (gateway *Gateway) callProvider(ctx context.Context, kind, baseURL, keyRef, model string, request Request) (string, error) {
	if kind != "openai_api" {
		return "", fmt.Errorf("codex_cli is not configured")
	}
	if baseURL == "" {
		return "", fmt.Errorf("missing base_url")
	}
	apiKey := os.Getenv(keyRef)
	if apiKey == "" {
		return "", fmt.Errorf("missing secret %s", keyRef)
	}
	payload, _ := json.Marshal(map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": request.System},
			{"role": "user", "content": request.Input},
		},
	})
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := gateway.client.Do(httpRequest)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		return "", fmt.Errorf("provider status %d", response.StatusCode)
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(response.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("empty completion")
	}
	return parsed.Choices[0].Message.Content, nil
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
		SELECT id, kind, COALESCE(base_url, ''), enabled, status, api_key_ref IS NOT NULL AND api_key_ref <> ''
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
