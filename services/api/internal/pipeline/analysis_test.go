package pipeline

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCleanArticleTextRemovesPublisherFurniture(t *testing.T) {
	input := `මාකුඹුරේ පරීක්ෂාවකදී පුද්ගලයන් නව දෙනකු අත්අඩංගුවට ගෙන තිබේ.

Share This Article Facebook Whatsapp Telegram Copy Link Print
Trending News unrelated headline`

	cleaned := cleanArticleText(input)

	require.Equal(t, "මාකුඹුරේ පරීක්ෂාවකදී පුද්ගලයන් නව දෙනකු අත්අඩංගුවට ගෙන තිබේ.", cleaned)
}

func TestCleanArticleTextNormalizesMarkupLinksAndWhitespace(t *testing.T) {
	input := `<p>The cabinet approved the proposal.</p><script>ignore()</script>
	Read more at https://example.com/story    More details follow.`

	cleaned := cleanArticleText(input)

	require.Equal(t, "The cabinet approved the proposal.\n\nRead more at More details follow.", cleaned)
}

func TestCleanArticleTextCutsInlineWebsiteNavigation(t *testing.T) {
	input := `දේශපාලන පුවතේ ප්‍රධාන කරුණු මෙහි විස්තර කර ඇත. තවත් වැදගත් තොරතුරුද මෙහි අඩංගුය.
Tags
politics Sri Lanka Previous article වෙනත් පුවත Share post: Facebook X Pinterest WhatsApp Popular තවත් සිරස්තලයක්`

	cleaned := cleanArticleText(input)

	require.Equal(t, "දේශපාලන පුවතේ ප්‍රධාන කරුණු මෙහි විස්තර කර ඇත. තවත් වැදගත් තොරතුරුද මෙහි අඩංගුය.", cleaned)
}

func TestParseArticleSummaryAcceptsStructuredSummary(t *testing.T) {
	result, err := parseArticleSummary(`{
	  "summary": "The cabinet approved the proposal after reviewing its financial impact.\n\nThe change will now proceed to Parliament."
	}`)

	require.NoError(t, err)
	require.Equal(t, "The cabinet approved the proposal after reviewing its financial impact.\n\nThe change will now proceed to Parliament.", result.Summary)
}

func TestCleanArticleTextPreservesReadableMarkdownAndRemovesArtifacts(t *testing.T) {
	input := "\ufeff<h2>Policy update</h2><p>The cabinet\u200b approved the plan � today.</p><p>Read: [source](https://example.com/story)</p>"

	cleaned := cleanArticleText(input)

	require.Equal(t, "## Policy update\n\nThe cabinet approved the plan today.\n\nRead: source", cleaned)
}

func TestParseCleanedArticleRejectsEmptyMarkdown(t *testing.T) {
	_, err := parseCleanedArticle(`{"markdown":"\u200b�"}`)

	require.Error(t, err)
}

func TestParseArticleSummaryConvertsListsIntoParagraphs(t *testing.T) {
	result, err := parseArticleSummary(`{"summary":"- Cabinet approved the plan.\n- Parliament will debate it tomorrow."}`)

	require.NoError(t, err)
	require.Equal(t, "Cabinet approved the plan. Parliament will debate it tomorrow.", result.Summary)
}

func TestParseEventSummaryRejectsEmptyOutput(t *testing.T) {
	_, err := parseEventSummary(`{"summary":"short"}`)

	require.Error(t, err)
}

func TestNormalizePercentagesTotalsOneHundred(t *testing.T) {
	left, center, right := normalizePercentages(0.333, 0.333, 0.334)

	require.Equal(t, 100.0, left+center+right)
	require.Equal(t, 33.3, left)
	require.Equal(t, 33.3, center)
	require.Equal(t, 33.4, right)
}
