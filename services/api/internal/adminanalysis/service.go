package adminanalysis

import (
	"context"
	"fmt"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm/codex"
)

type Service struct {
	store *Store
	model llm.Completer
	codex *codex.Client
}

func NewService(store *Store, model llm.Completer, codex *codex.Client) *Service {
	return &Service{store: store, model: model, codex: codex}
}

func (service *Service) Store() *Store { return service.store }

func (service *Service) CodexStatus(ctx context.Context) codex.Status {
	return service.codex.Probe(ctx)
}

func (service *Service) Analyze(ctx context.Context, runID, articleID, provider, model string) error {
	article, err := service.store.Article(ctx, articleID)
	if err != nil {
		return err
	}
	input := BuildArticleInput(article)
	response, err := service.model.CompleteWithModel(ctx, llm.Request{
		Task: "admin_article_analysis", System: SystemPrompt, Input: input,
		JSONSchema: InsightSchema, DisableReasoning: true, MaxTokens: 1800,
		ArticleID: articleID,
	}, provider, model)
	if err != nil {
		return fmt.Errorf("run %s article analysis: %w", provider, err)
	}
	if response.Provider == "none" {
		return fmt.Errorf("run %s article analysis: no model provider responded", provider)
	}
	insight, err := ParseInsight(response.Text)
	if err != nil {
		return err
	}
	return service.store.SaveInsight(ctx, runID, articleID, response.Provider, response.Model, insight)
}
