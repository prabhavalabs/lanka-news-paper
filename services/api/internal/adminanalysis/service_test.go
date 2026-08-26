package adminanalysis

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidateCreateRequestAcceptsDateRange(t *testing.T) {
	from := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)

	err := ValidateCreateRequest(CreateRequest{
		Scope: "date_range", Provider: "openrouter", Model: "deepseek/deepseek-v3.1-terminus",
		From: &from, To: &to,
	})

	require.NoError(t, err)
}

func TestValidateCreateRequestRequiresCatalogConfirmation(t *testing.T) {
	err := ValidateCreateRequest(CreateRequest{
		Scope: "catalog", Provider: "codex_cli", Model: "gpt-5.6-luna",
	})

	require.EqualError(t, err, `confirmation must be "BACKFILL ENTIRE CATALOG" for a catalog backfill`)
}

func TestValidateCreateRequestRejectsUnsupportedProvider(t *testing.T) {
	err := ValidateCreateRequest(CreateRequest{
		Scope: "article", Provider: "unknown", Model: "model", ArticleID: "4dc5d133-073d-40a8-95ac-7d9871991b4a",
	})

	require.EqualError(t, err, "provider must be openrouter or codex_cli")
}

func TestValidateCreateRequestAcceptsFullEditorialPipeline(t *testing.T) {
	err := ValidateCreateRequest(CreateRequest{
		Scope: "article", Workflow: "full_pipeline", Provider: "codex_cli",
		Model: "gpt-5.6-terra", ArticleID: "4dc5d133-073d-40a8-95ac-7d9871991b4a",
	})

	require.NoError(t, err)
}

func TestValidateCreateRequestRejectsPipelineProviderForSinglePass(t *testing.T) {
	err := ValidateCreateRequest(CreateRequest{
		Scope: "article", Workflow: "single_pass", Provider: "pipeline",
		Model: "configured-profiles", ArticleID: "4dc5d133-073d-40a8-95ac-7d9871991b4a",
	})

	require.EqualError(t, err, "provider must be openrouter or codex_cli")
}

func TestParseInsightAcceptsStrictStructuredOutput(t *testing.T) {
	result, err := ParseInsight(`{
		"summary":"The cabinet approved a targeted tax change and scheduled parliamentary debate.",
		"tone":"neutral",
		"political_relevance":true,
		"political_narrative":"The report presents the change as a fiscal consolidation measure.",
		"spectrum_score":0.2,
		"confidence":0.84,
		"evidence":["approved a targeted tax change"]
	}`)

	require.NoError(t, err)
	require.Equal(t, "neutral", result.Tone)
	require.InDelta(t, 0.2, result.SpectrumScore, 0.001)
}

func TestParseInsightRejectsOutOfRangeScore(t *testing.T) {
	_, err := ParseInsight(`{
		"summary":"A precise summary that is long enough to be useful.",
		"tone":"negative",
		"political_relevance":true,
		"political_narrative":"A narrative.",
		"spectrum_score":1.5,
		"confidence":0.8,
		"evidence":[]
	}`)

	require.ErrorContains(t, err, "spectrum_score")
}

func TestBuildArticleInputPrefersFullContentAndLimitsSize(t *testing.T) {
	body := strings.Repeat("අ", maxArticleCharacters+100)
	input := BuildArticleInput(ArticleInput{Headline: "Headline", Description: "Excerpt", Body: body})

	require.Contains(t, input, `"headline":"Headline"`)
	require.NotContains(t, input, "Excerpt")
	require.LessOrEqual(t, len([]rune(input)), maxArticleCharacters+200)
}
