package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
)

func (store *Store) cleanContent(ctx context.Context, articleID, runID, stepID string, selection modelSelection) (stepResult, error) {
	var sourceContentID *string
	var original string
	if err := store.pool.QueryRow(ctx, `
		SELECT content.id::text,
		       COALESCE(content.body_text, NULLIF(article.description, ''), article.headline)
		FROM articles article
		LEFT JOIN LATERAL (
		  SELECT id, body_text FROM article_contents
		  WHERE article_id = article.id AND current
		  ORDER BY version DESC LIMIT 1
		) content ON true
		WHERE article.id = $1
	`, articleID).Scan(&sourceContentID, &original); err != nil {
		return stepResult{}, fmt.Errorf("load article content for cleaning: %w", err)
	}
	cleaned := cleanArticleText(original)
	if cleaned == "" {
		return stepResult{}, fmt.Errorf("article content is empty after cleaning")
	}
	provider, model := "deterministic", cleanerVersion
	payload, err := json.Marshal(map[string]string{"article": truncate(cleaned, 45000)})
	if err != nil {
		return stepResult{}, err
	}
	response, err := store.complete(ctx, llm.Request{
		Task: "content_cleaning", System: cleanedArticlePrompt, Input: string(payload),
		JSONSchema: cleanedArticleSchema, DisableReasoning: true, MaxTokens: 12000,
		ArticleID: articleID, PipelineRunID: runID, PipelineStepID: stepID,
	}, selection)
	if err != nil {
		return stepResult{}, fmt.Errorf("clean article with %s: %w", selection.provider, err)
	}
	if response.Provider != "none" {
		result, err := parseCleanedArticle(response.Text)
		if err != nil {
			return stepResult{}, err
		}
		cleaned, provider, model = result.Markdown, response.Provider, response.Model
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO article_analysis_documents (
		  article_id, source_content_id, original_text, cleaned_text, cleaner_version,
		  cleaner_provider, cleaner_model
		) VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, $6, $7)
		ON CONFLICT (article_id) DO UPDATE SET
		  source_content_id = EXCLUDED.source_content_id,
		  original_text = EXCLUDED.original_text,
		  cleaned_text = EXCLUDED.cleaned_text,
		  summary_text = CASE
		    WHEN article_analysis_documents.cleaned_text IS DISTINCT FROM EXCLUDED.cleaned_text THEN ''
		    ELSE article_analysis_documents.summary_text END,
		  summary_points = CASE
		    WHEN article_analysis_documents.cleaned_text IS DISTINCT FROM EXCLUDED.cleaned_text THEN '[]'::jsonb
		    ELSE article_analysis_documents.summary_points END,
		  summary_provider = CASE
		    WHEN article_analysis_documents.cleaned_text IS DISTINCT FROM EXCLUDED.cleaned_text THEN ''
		    ELSE article_analysis_documents.summary_provider END,
		  summary_model = CASE
		    WHEN article_analysis_documents.cleaned_text IS DISTINCT FROM EXCLUDED.cleaned_text THEN ''
		    ELSE article_analysis_documents.summary_model END,
		  summarized_at = CASE
		    WHEN article_analysis_documents.cleaned_text IS DISTINCT FROM EXCLUDED.cleaned_text THEN NULL
		    ELSE article_analysis_documents.summarized_at END,
		  cleaner_version = EXCLUDED.cleaner_version,
		  cleaner_provider = EXCLUDED.cleaner_provider,
		  cleaner_model = EXCLUDED.cleaner_model,
		  cleaned_at = clock_timestamp(),
		  updated_at = clock_timestamp()
	`, articleID, valueOrEmpty(sourceContentID), original, cleaned, cleanerVersion, provider, model); err != nil {
		return stepResult{}, fmt.Errorf("save cleaned article: %w", err)
	}
	return stepResult{output: map[string]any{
		"cleaner":             cleanerVersion,
		"provider":            provider,
		"model":               model,
		"original_characters": len([]rune(original)),
		"cleaned_characters":  len([]rune(cleaned)),
		"removed_characters":  len([]rune(original)) - len([]rune(cleaned)),
	}}, nil
}

func (store *Store) summarize(ctx context.Context, articleID, runID, stepID string, selection modelSelection) (stepResult, error) {
	var headline, cleaned string
	if err := store.pool.QueryRow(ctx, `
		SELECT article.headline, document.cleaned_text
		FROM articles article
		JOIN article_analysis_documents document ON document.article_id = article.id
		WHERE article.id = $1
	`, articleID).Scan(&headline, &cleaned); err != nil {
		return stepResult{}, fmt.Errorf("load cleaned article for summarization: %w", err)
	}
	input, err := json.Marshal(map[string]string{
		"headline": headline,
		"article":  truncate(cleaned, 16000),
	})
	if err != nil {
		return stepResult{}, err
	}
	response, err := store.complete(ctx, llm.Request{
		Task: "article_summary", System: articleSummaryPrompt, Input: string(input),
		JSONSchema: articleSummarySchema, DisableReasoning: true, MaxTokens: 1800,
		ArticleID: articleID, PipelineRunID: runID, PipelineStepID: stepID,
	}, selection)
	if err != nil {
		return stepResult{}, fmt.Errorf("summarize article: %w", err)
	}
	if response.Provider == "none" {
		return stepResult{}, fmt.Errorf("summarize article: no model provider responded")
	}
	summary, err := parseArticleSummary(response.Text)
	if err != nil {
		return stepResult{}, err
	}
	if _, err := store.pool.Exec(ctx, `
		UPDATE article_analysis_documents
		SET summary_text = $2, summary_points = $3, summary_provider = $4,
		    summary_model = $5, summarized_at = clock_timestamp(), updated_at = clock_timestamp()
		WHERE article_id = $1
	`, articleID, summary.Summary, []byte("[]"), response.Provider, response.Model); err != nil {
		return stepResult{}, fmt.Errorf("save article summary: %w", err)
	}
	return stepResult{output: map[string]any{
		"summary":  summary.Summary,
		"provider": response.Provider, "model": response.Model,
	}}, nil
}

type eventMember struct {
	ArticleID  string
	SourceID   string
	Source     string
	Icon       string
	Headline   string
	Summary    string
	Relevant   *bool
	Left       *float64
	Center     *float64
	Right      *float64
	Confidence *float64
}

type sourceSpectrumItem struct {
	ArticleID         string  `json:"article_id"`
	SourceID          string  `json:"source_id"`
	Source            string  `json:"source"`
	SourceIcon        string  `json:"source_icon"`
	Label             string  `json:"label"`
	LeftProbability   float64 `json:"left_probability"`
	CenterProbability float64 `json:"center_probability"`
	RightProbability  float64 `json:"right_probability"`
	Confidence        float64 `json:"confidence"`
}

func (store *Store) synthesizeEvent(ctx context.Context, articleID, runID, stepID string, selection modelSelection) (stepResult, error) {
	var eventID *string
	if err := store.pool.QueryRow(ctx, `SELECT event_id::text FROM articles WHERE id = $1`, articleID).Scan(&eventID); err != nil {
		return stepResult{}, fmt.Errorf("load article event: %w", err)
	}
	if eventID == nil {
		return stepResult{output: map[string]any{"reason": "Article is not attached to an event."}, skipped: true}, nil
	}
	members, err := store.eventMembers(ctx, *eventID)
	if err != nil {
		return stepResult{}, err
	}
	if len(members) == 0 {
		return stepResult{output: map[string]any{"reason": "Event has no public articles."}, skipped: true}, nil
	}
	var articleCount int
	if err := store.pool.QueryRow(ctx, `
		SELECT count(*) FROM articles WHERE event_id = $1 AND public_status = 'published'
	`, *eventID).Scan(&articleCount); err != nil {
		return stepResult{}, fmt.Errorf("count event coverage: %w", err)
	}

	spectrum := make([]sourceSpectrumItem, 0, len(members))
	sourceIDs := make(map[string]struct{}, len(members))
	leftTotal, centerTotal, rightTotal, rated := 0.0, 0.0, 0.0, 0
	for _, member := range members {
		sourceIDs[member.SourceID] = struct{}{}
		item := sourceSpectrumItem{
			ArticleID: member.ArticleID, SourceID: member.SourceID, Source: member.Source,
			SourceIcon: member.Icon, Label: "unrated", CenterProbability: 1,
		}
		if member.Relevant != nil && *member.Relevant && member.Left != nil && member.Center != nil && member.Right != nil {
			item.LeftProbability, item.CenterProbability, item.RightProbability = *member.Left, *member.Center, *member.Right
			item.Label = dominantStance(*member.Left, *member.Center, *member.Right)
			if member.Confidence != nil {
				item.Confidence = *member.Confidence
			}
			leftTotal += *member.Left
			centerTotal += *member.Center
			rightTotal += *member.Right
			rated++
		}
		spectrum = append(spectrum, item)
	}
	leftPercentage, centerPercentage, rightPercentage := normalizePercentages(leftTotal, centerTotal, rightTotal)
	if rated == 0 {
		leftPercentage, centerPercentage, rightPercentage = 0, 0, 0
	}

	summary, provider, model, err := store.crossSourceSummary(ctx, articleID, runID, stepID, members, selection)
	if err != nil {
		return stepResult{}, err
	}
	spectrumJSON, err := json.Marshal(spectrum)
	if err != nil {
		return stepResult{}, err
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO event_narrative_analyses (
		  event_id, summary, article_count, source_count, rated_source_count,
		  left_percentage, center_percentage, right_percentage, source_spectrum,
		  provider_id, provider_model
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (event_id) DO UPDATE SET
		  summary = EXCLUDED.summary,
		  article_count = EXCLUDED.article_count,
		  source_count = EXCLUDED.source_count,
		  rated_source_count = EXCLUDED.rated_source_count,
		  left_percentage = EXCLUDED.left_percentage,
		  center_percentage = EXCLUDED.center_percentage,
		  right_percentage = EXCLUDED.right_percentage,
		  source_spectrum = EXCLUDED.source_spectrum,
		  provider_id = EXCLUDED.provider_id,
		  provider_model = EXCLUDED.provider_model,
		  analyzed_at = clock_timestamp()
	`, *eventID, summary, articleCount, len(sourceIDs), rated, leftPercentage, centerPercentage,
		rightPercentage, spectrumJSON, provider, model); err != nil {
		return stepResult{}, fmt.Errorf("save event narrative analysis: %w", err)
	}
	return stepResult{output: map[string]any{
		"event_id": *eventID, "summary": summary, "article_count": articleCount,
		"source_count": len(sourceIDs), "rated_source_count": rated,
		"left_percentage": leftPercentage, "center_percentage": centerPercentage,
		"right_percentage": rightPercentage, "source_spectrum": spectrum,
		"provider": provider, "model": model,
	}}, nil
}

func (store *Store) eventMembers(ctx context.Context, eventID string) ([]eventMember, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT DISTINCT ON (article.source_id)
		       article.id::text, source.id::text, source.name, COALESCE(source.icon_url, ''),
		       article.headline, COALESCE(document.summary_text, ''), analysis.relevant,
		       analysis.left_probability, analysis.center_probability,
		       analysis.right_probability, analysis.confidence
		FROM articles article
		JOIN sources source ON source.id = article.source_id
		LEFT JOIN article_analysis_documents document ON document.article_id = article.id
		LEFT JOIN article_political_analysis analysis ON analysis.article_id = article.id
		WHERE article.event_id = $1 AND article.public_status = 'published'
		ORDER BY article.source_id, article.published_at DESC, article.id
	`, eventID)
	if err != nil {
		return nil, fmt.Errorf("load event coverage for synthesis: %w", err)
	}
	defer rows.Close()
	members := make([]eventMember, 0)
	for rows.Next() {
		var member eventMember
		if err := rows.Scan(
			&member.ArticleID, &member.SourceID, &member.Source, &member.Icon,
			&member.Headline, &member.Summary, &member.Relevant, &member.Left,
			&member.Center, &member.Right, &member.Confidence,
		); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (store *Store) crossSourceSummary(ctx context.Context, articleID, runID, stepID string, members []eventMember, selection modelSelection) (string, string, string, error) {
	if len(members) == 1 {
		summary := strings.TrimSpace(members[0].Summary)
		if summary == "" {
			summary = members[0].Headline
		}
		return summary, "derived", "single-source-summary-v1", nil
	}
	type sourceSummary struct {
		Source   string `json:"source"`
		Headline string `json:"headline"`
		Summary  string `json:"summary"`
	}
	inputs := make([]sourceSummary, 0, len(members))
	for _, member := range members {
		summary := strings.TrimSpace(member.Summary)
		if summary == "" {
			summary = member.Headline
		}
		inputs = append(inputs, sourceSummary{Source: member.Source, Headline: member.Headline, Summary: summary})
	}
	payload, err := json.Marshal(map[string]any{"source_reports": inputs})
	if err != nil {
		return "", "", "", err
	}
	response, err := store.complete(ctx, llm.Request{
		Task: "event_synthesis", System: eventSummaryPrompt, Input: string(payload),
		JSONSchema: eventSummarySchema, DisableReasoning: true, MaxTokens: 2200,
		ArticleID: articleID, PipelineRunID: runID, PipelineStepID: stepID,
	}, selection)
	if err != nil {
		return "", "", "", fmt.Errorf("synthesize event coverage: %w", err)
	}
	if response.Provider == "none" {
		return "", "", "", fmt.Errorf("synthesize event coverage: no model provider responded")
	}
	result, err := parseEventSummary(response.Text)
	if err != nil {
		return "", "", "", err
	}
	return result.Summary, response.Provider, response.Model, nil
}

func dominantStance(left, center, right float64) string {
	if left > center && left > right {
		return "left"
	}
	if right > center && right > left {
		return "right"
	}
	return "center"
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
