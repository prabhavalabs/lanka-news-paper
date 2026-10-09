package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm/codex"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/politics"
	"github.com/stretchr/testify/require"
)

const testNarration = `{"political_relevance":1,"left_probability":0.1,"center_probability":0.8,"right_probability":0.1,"confidence":0.85,"rationale":"Neutral attribution of the reported decision.","evidence":[]}`

func combinedFixture(article string) string {
	data, _ := json.Marshal(map[string]any{
		"cleaned":   map[string]string{"markdown": article},
		"summary":   map[string]string{"summary": article},
		"category":  map[string]any{"slug": "latest", "confidence": 0.72},
		"narration": json.RawMessage(testNarration),
	})
	return string(data)
}

func TestArticleAnalysisPromptSchema(t *testing.T) {
	prompt, schema := articleAnalysisPromptSchema()
	narrationPrompt, narrationSchema := politics.PromptSchema()
	for _, existing := range []string{cleanedArticlePrompt, articleSummaryPrompt, classificationPrompt, narrationPrompt} {
		require.Contains(t, prompt, existing, "existing editorial instructions must remain verbatim")
	}
	require.Contains(t, prompt, "first 16000 characters of that cleaned text")
	require.Contains(t, prompt, "without economic-policy signals")
	require.Equal(t, false, schema["additionalProperties"])
	require.ElementsMatch(t, []string{"cleaned", "summary", "category", "narration"}, schema["required"])
	properties := schema["properties"].(map[string]any)
	require.Equal(t, cleanedArticleSchema, properties["cleaned"])
	require.Equal(t, articleSummarySchema, properties["summary"])
	require.Equal(t, narrationSchema, properties["narration"])
	require.Equal(t, []string{"latest", "politics", "economy", "world", "crime", "health", "environment", "sport", "education", "entertainment", "technology"}, classificationSlugs())
}

func TestArticleAnalysisInputBoundsRunes(t *testing.T) {
	input, err := articleAnalysisInput(strings.Repeat("අ", 2000), strings.Repeat("ක", 46000))
	require.NoError(t, err)
	var decoded map[string]string
	require.NoError(t, json.Unmarshal([]byte(input), &decoded))
	require.Len(t, []rune(decoded["headline"]), 1800)
	require.Len(t, []rune(decoded["article"]), 45000)
}

func TestCombinedAnalysisParsesPartsIndependently(t *testing.T) {
	valid := combinedFixture("A community gathering concluded peacefully with participants returning home.")
	for _, test := range []struct{ name, part, value string }{
		{"valid", "", ""},
		{"invalid cleaning", "cleaned", `{"markdown":""}`},
		{"invalid summary", "summary", `{"summary":"short"}`},
		{"invalid category", "category", `{"slug":"local","confidence":0.8}`},
		{"missing confidence", "category", `{"slug":"economy"}`},
		{"invalid confidence", "category", `{"slug":"economy","confidence":1.1}`},
		{"invalid narration", "narration", `{"political_relevance":1,"left_probability":0.5,"center_probability":0.8,"right_probability":0.1,"confidence":0.8}`},
		{"missing narration", "narration", `null`},
		{"missing narration confidence", "narration", `{"political_relevance":1,"left_probability":0.1,"center_probability":0.8,"right_probability":0.1,"rationale":"neutral","evidence":[]}`},
		{"unknown narration field", "narration", `{"political_relevance":1,"left_probability":0.1,"center_probability":0.8,"right_probability":0.1,"confidence":0.8,"rationale":"neutral","evidence":[],"narration_score":0}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var parts map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(valid), &parts))
			if test.part != "" {
				parts[test.part] = json.RawMessage(test.value)
			}
			data, err := json.Marshal(parts)
			require.NoError(t, err)
			analysis, err := parseCombinedAnalysis(llm.Response{Text: string(data), Provider: "codex_cli", Model: "gpt-6-luna"})
			require.NoError(t, err)
			for _, part := range []string{"cleaned", "summary", "category", "narration"} {
				_, valid := analysis.response(part)
				require.Equal(t, part != test.part, valid, part)
			}
		})
	}
	for _, bad := range []string{`not JSON`, valid + ` {}`, `{"unknown":{}}`} {
		_, err := parseCombinedAnalysis(llm.Response{Text: bad})
		require.Error(t, err)
	}
	analysis, err := parseCombinedAnalysis(llm.Response{Text: valid, Provider: "none"})
	require.NoError(t, err)
	_, usable := analysis.response("summary")
	require.False(t, usable)
}

type countingCompleter struct {
	calls       []string
	pinned      []string
	combined    string
	fail        bool
	failSummary bool
	article     string
	requests    []llm.Request
}

func (model *countingCompleter) Complete(_ context.Context, request llm.Request) (llm.Response, error) {
	model.calls = append(model.calls, request.Task)
	model.requests = append(model.requests, request)
	response := llm.Response{Provider: "codex_cli", Model: "gpt-6-luna"}
	switch request.Task {
	case "article_analysis":
		if model.fail {
			return llm.Response{Provider: "none"}, nil
		}
		response.Text = model.combined
	case "content_cleaning":
		data, _ := json.Marshal(cleanedArticle{Markdown: model.article})
		response.Text = string(data)
	case "article_summary":
		if model.failSummary {
			return llm.Response{}, fmt.Errorf("interrupted summary")
		}
		data, _ := json.Marshal(articleSummary{Summary: model.article})
		response.Text = string(data)
	case "classify":
		response.Text = "latest"
	case "narration_framing":
		response.Text = testNarration
	default:
		return llm.Response{}, fmt.Errorf("unexpected task %s", request.Task)
	}
	return response, nil
}

func (model *countingCompleter) CompleteWithModel(ctx context.Context, request llm.Request, provider, name string) (llm.Response, error) {
	model.pinned = append(model.pinned, provider+":"+name)
	response, err := model.Complete(ctx, request)
	if response.Provider != "none" {
		response.Provider, response.Model = provider, name
	}
	return response, err
}

func TestCombinedStepReuseAndStandaloneFallback(t *testing.T) {
	article := "A community gathering concluded peacefully with participants returning home."
	for _, test := range []struct {
		name           string
		cached         bool
		invalidSummary bool
		wantCalls      []string
	}{
		{"cached", true, false, []string{"article_analysis"}},
		{"new manual run", false, false, []string{"content_cleaning", "article_summary", "classify", "narration_framing"}},
		{"one invalid part", true, true, []string{"article_analysis", "article_summary"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := &countingCompleter{article: article, combined: combinedFixture(article)}
			store := NewStore(nil, model, nil, nil)
			var cached *combinedAnalysis
			if test.cached {
				response, err := store.complete(context.Background(), llm.Request{Task: "article_analysis"}, modelSelection{})
				require.NoError(t, err)
				analysis, err := parseCombinedAnalysis(response)
				require.NoError(t, err)
				cached = &analysis
				if test.invalidSummary {
					cached.Parts["summary"] = json.RawMessage(`{"summary":"short"}`)
				}
			}
			for _, part := range []struct{ part, task string }{{"cleaned", "content_cleaning"}, {"summary", "article_summary"}, {"category", "classify"}, {"narration", "narration_framing"}} {
				response, reused, err := store.resolvePart(context.Background(), cached, part.part, llm.Request{Task: part.task}, modelSelection{})
				require.NoError(t, err)
				require.Equal(t, test.cached && !(test.invalidSummary && part.part == "summary"), reused)
				require.Equal(t, "codex_cli", response.Provider)
			}
			require.Equal(t, test.wantCalls, model.calls)
		})
	}
}

func TestArticleAnalysisCodexSmoke(t *testing.T) {
	if os.Getenv("SNAP_CODEX_SMOKE") != "1" {
		t.Skip("explicit live Codex smoke is not enabled")
	}
	article := `අඩු ආදායම්ලාභී පවුල් සඳහා ලබාදෙන සුබසාධන දීමනා ලබන මාසයේ සිට වැඩි කිරීමට යෝජනා කර ඇති බව මුදල් අමාත්‍යාංශය සඳහන් කළේය. එම වැඩසටහන සඳහා රුපියල් බිලියන 12ක් වෙන් කිරීමට සැලසුම් කර ඇති අතර, අවසන් අනුමැතිය පාර්ලිමේන්තුවෙන් ලබාගත යුතු බව නිලධාරියෙක් පැවසීය. බදු ආදායම වැඩි කිරීමෙන් අමතර වියදම පියවා ගැනීමට අපේක්ෂා කරන බව අමාත්‍යාංශය කියයි. යෝජනාව පිළිබඳ විස්තරාත්මක අයවැය ලේඛනය තවමත් ප්‍රකාශයට පත් කර නොමැත.`
	prompt, schema := articleAnalysisPromptSchema()
	input, err := json.Marshal(map[string]string{"headline": "සුබසාධන දීමනා වැඩි කිරීමට යෝජනාවක්", "article": article})
	require.NoError(t, err)
	started := time.Now()
	result, err := codex.NewClient(nil).Complete(context.Background(), codex.CallOptions{
		Model: "gpt-6-luna", ReasoningEffort: "medium", System: prompt, Input: string(input), JSONSchema: schema, Timeout: 240 * time.Second,
	})
	t.Logf("latency_ms=%d input_tokens=%d cached_input_tokens=%d output_tokens=%d reasoning_tokens=%d", time.Since(started).Milliseconds(), result.InputTokens, result.CachedInputTokens, result.OutputTokens, result.ReasoningTokens)
	require.NoError(t, err)
	analysis, err := parseCombinedAnalysis(llm.Response{Text: result.Text, Provider: "codex_cli", Model: "gpt-6-luna"})
	require.NoError(t, err)
	for _, part := range []string{"cleaned", "summary", "category", "narration"} {
		_, valid := analysis.response(part)
		t.Logf("%s_valid=%t", part, valid)
		require.True(t, valid, part)
	}
	summary, err := parseArticleSummary(string(analysis.Parts["summary"]))
	require.NoError(t, err)
	require.True(t, strings.Contains(summary.Summary, "යෝජ") || strings.Contains(summary.Summary, "අනුමැතිය"), "summary must preserve proposed status")
}
