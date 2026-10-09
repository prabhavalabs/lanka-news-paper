package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/politics"
)

type articleCategory struct {
	Slug       string   `json:"slug"`
	Confidence *float64 `json:"confidence"`
}

// The raw sub-objects are retained so each step uses its existing parser.
// Provider "none" also records an attempted call, preventing a retry of the
// cleaning step from spending another combined call in the same run.
type combinedAnalysis struct {
	CallID   int64                      `json:"-"`
	Parts    map[string]json.RawMessage `json:"parts"`
	Provider string                     `json:"provider"`
	Model    string                     `json:"model"`
}

func classificationSlugs() []string {
	return strings.Split(strings.TrimSuffix(strings.Split(classificationPrompt, "\n")[1], "."), ", ")
}

func articleAnalysisPromptSchema() (string, map[string]any) {
	narrationPrompt, narrationSchema := politics.PromptSchema()
	prompt := `Analyze one article in a single pass. Return one JSON object with cleaned, summary, category, and narration sub-objects. The rules below apply to their named sub-object; the combined JSON format supersedes any section's standalone output format.
First produce cleaned.markdown from the supplied article. Summarize only the first 16000 characters of that cleaned text. Categorize its meaning. Evaluate narration using the headline, the first 1800 characters of your summary, and the first 6000 characters of the cleaned text. Do not use removed scraper debris as evidence. A report without economic-policy signals must have political_relevance=0, left_probability=0, center_probability=1, right_probability=0. Category must contain slug and confidence (0 to 1).

CLEANED:
` + cleanedArticlePrompt + "\n\nSUMMARY:\n" + articleSummaryPrompt + "\n\nCATEGORY:\n" + classificationPrompt + "\n\nNARRATION:\n" + narrationPrompt
	return prompt, map[string]any{
		"type": "object", "additionalProperties": false,
		"required": []string{"cleaned", "summary", "category", "narration"},
		"properties": map[string]any{
			"cleaned": cleanedArticleSchema, "summary": articleSummarySchema,
			"category": map[string]any{
				"type": "object", "additionalProperties": false,
				"required": []string{"slug", "confidence"},
				"properties": map[string]any{
					"slug":       map[string]any{"type": "string", "enum": classificationSlugs()},
					"confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
				},
			},
			"narration": narrationSchema,
		},
	}
}

func articleAnalysisInput(headline, article string) (string, error) {
	data, err := json.Marshal(map[string]string{"headline": truncate(headline, 1800), "article": truncate(article, 45000)})
	return string(data), err
}

func parseCombinedAnalysis(response llm.Response) (combinedAnalysis, error) {
	result := combinedAnalysis{Provider: response.Provider, Model: response.Model}
	var parts struct {
		Cleaned   json.RawMessage `json:"cleaned"`
		Summary   json.RawMessage `json:"summary"`
		Category  json.RawMessage `json:"category"`
		Narration json.RawMessage `json:"narration"`
	}
	if err := decodeStructured(response.Text, &parts); err != nil {
		return result, fmt.Errorf("decode combined analysis: %w", err)
	}
	result.Parts = map[string]json.RawMessage{
		"cleaned": parts.Cleaned, "summary": parts.Summary,
		"category": parts.Category, "narration": parts.Narration,
	}
	return result, nil
}

func parseArticleCategory(value string) (articleCategory, error) {
	var result articleCategory
	if err := decodeStructured(value, &result); err != nil {
		return result, err
	}
	valid := false
	for _, slug := range classificationSlugs() {
		valid = valid || result.Slug == slug
	}
	if !valid || result.Confidence == nil || *result.Confidence < 0 || *result.Confidence > 1 {
		return result, fmt.Errorf("invalid combined category slug or confidence")
	}
	return result, nil
}

func (analysis combinedAnalysis) response(part string) (llm.Response, bool) {
	response := llm.Response{Text: string(analysis.Parts[part]), Provider: analysis.Provider, Model: analysis.Model, CallID: analysis.CallID}
	if analysis.Provider == "" || analysis.Provider == "none" {
		return response, false
	}
	// Match the strict combined schema before applying the standalone parsers,
	// which also support historical narration formats and normalization.
	_, schema := articleAnalysisPromptSchema()
	partSchema, known := schema["properties"].(map[string]any)[part]
	if !known || !requiredFieldsPresent(response.Text, partSchema.(map[string]any)) {
		return response, false
	}
	var err error
	switch part {
	case "cleaned":
		_, err = parseCleanedArticle(response.Text)
	case "summary":
		_, err = parseArticleSummary(response.Text)
	case "category":
		_, err = parseArticleCategory(response.Text)
	case "narration":
		_, err = politics.ParseAnalysis(response.Text)
	default:
		return response, false
	}
	return response, err == nil
}

func (store *Store) loadCombinedAnalysis(ctx context.Context, runID string) (*combinedAnalysis, error) {
	var data []byte
	var callID int64
	if err := store.pool.QueryRow(ctx, `SELECT combined_analysis, COALESCE(combined_analysis_call_id,0) FROM article_pipeline_runs WHERE id=$1`, runID).Scan(&data, &callID); err != nil {
		return nil, fmt.Errorf("load combined analysis: %w", err)
	}
	if len(data) == 0 {
		return nil, nil
	}
	var result combinedAnalysis
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode stored combined analysis: %w", err)
	}
	result.CallID = callID
	return &result, nil
}

func requiredFieldsPresent(value string, schema map[string]any) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(value), &fields); err != nil {
		return false
	}
	for _, key := range schema["required"].([]string) {
		if raw, ok := fields[key]; !ok || string(raw) == "null" {
			return false
		}
	}
	properties := schema["properties"].(map[string]any)
	for key := range fields {
		if _, ok := properties[key]; !ok {
			return false
		}
	}
	return true
}

func (store *Store) saveCombinedAnalysis(ctx context.Context, runID string, result combinedAnalysis, callID int64) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = store.pool.Exec(ctx, `UPDATE article_pipeline_runs SET combined_analysis=$2,
		combined_analysis_call_id=NULLIF($3, 0), updated_at=clock_timestamp() WHERE id=$1`, runID, data, callID)
	return err
}

func (store *Store) logCombinedReuse(ctx context.Context, runID, stepID, part string, response llm.Response) {
	store.appendLog(ctx, runID, stepID, "info", "reused_combined_analysis", "Reused article analysis", map[string]any{
		"part": part, "provider": response.Provider, "model": response.Model, "call_id": response.CallID,
	})
}

// partResponse is shared by the cleaning and summary steps. An invalid
// sub-object causes only that step's standalone call to run.
func (store *Store) partResponse(ctx context.Context, analysis *combinedAnalysis, part string, request llm.Request, selection modelSelection) (llm.Response, error) {
	response, reused, err := store.resolvePart(ctx, analysis, part, request, selection)
	if reused {
		store.logCombinedReuse(ctx, request.PipelineRunID, request.PipelineStepID, part, response)
	}
	return response, err
}

func (store *Store) resolvePart(ctx context.Context, analysis *combinedAnalysis, part string, request llm.Request, selection modelSelection) (llm.Response, bool, error) {
	if analysis != nil {
		if response, valid := analysis.response(part); valid {
			return response, true, nil
		}
	}
	response, err := store.complete(ctx, request, selection)
	return response, false, err
}
