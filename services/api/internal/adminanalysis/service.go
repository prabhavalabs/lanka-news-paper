package adminanalysis

import (
	"context"
	"fmt"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
)

type OpenRouterCompleter interface {
	CompleteWithModel(context.Context, llm.Request, string, string) (llm.Response, error)
}

type Service struct {
	store      *Store
	openRouter OpenRouterCompleter
	codex      *CodexClient
}

func NewService(store *Store, openRouter OpenRouterCompleter, codex *CodexClient) *Service {
	return &Service{store: store, openRouter: openRouter, codex: codex}
}

func (service *Service) Store() *Store { return service.store }

func (service *Service) CodexStatus(ctx context.Context) CodexStatus {
	return service.codex.Probe(ctx)
}

func (service *Service) Analyze(ctx context.Context, runID, articleID, provider, model string) error {
	article, err := service.store.Article(ctx, articleID)
	if err != nil {
		return err
	}
	input := BuildArticleInput(article)
	var output string
	switch provider {
	case "openrouter":
		response, err := service.openRouter.CompleteWithModel(ctx, llm.Request{
			Task: "admin_article_analysis", System: SystemPrompt, Input: input,
			JSONSchema: InsightSchema, DisableReasoning: true, MaxTokens: 1800,
			ArticleID: articleID,
		}, "openrouter", model)
		if err != nil {
			return fmt.Errorf("run OpenRouter article analysis: %w", err)
		}
		output = response.Text
	case "codex_cli":
		output, err = service.completeWithCodex(ctx, articleID, model, input)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported administrative analysis provider %q", provider)
	}
	insight, err := ParseInsight(output)
	if err != nil {
		return err
	}
	return service.store.SaveInsight(ctx, runID, articleID, provider, model, insight)
}

func (service *Service) completeWithCodex(ctx context.Context, articleID, model, input string) (string, error) {
	started := time.Now()
	callID := service.startCodexCall(ctx, articleID, model)
	output, err := service.codex.Complete(ctx, model, SystemPrompt+"\n\nArticle JSON:\n"+input, InsightSchema)
	service.finishCodexCall(context.WithoutCancel(ctx), callID, time.Since(started), output, err)
	if err != nil {
		return "", err
	}
	return output, nil
}

func (service *Service) startCodexCall(ctx context.Context, articleID, model string) int64 {
	var id int64
	_ = service.store.pool.QueryRow(ctx, `
		INSERT INTO llm_calls (task, provider_id, model, outcome, article_id, streamed)
		VALUES ('admin_article_analysis', 'codex_cli', $2, 'running', $1, false)
		RETURNING id
	`, articleID, model).Scan(&id)
	return id
}

func (service *Service) finishCodexCall(ctx context.Context, id int64, duration time.Duration, output string, callErr error) {
	if id == 0 {
		return
	}
	outcome, detail := "ok", ""
	if callErr != nil {
		outcome, detail = "failed", callErr.Error()
	}
	recordContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, _ = service.store.pool.Exec(recordContext, `
		UPDATE llm_calls
		SET outcome = $2, latency_ms = $3, response_text = NULLIF($4, ''),
		    error_detail = NULLIF($5, ''), completed_at = clock_timestamp()
		WHERE id = $1
	`, id, outcome, duration.Milliseconds(), output, detail)
}
