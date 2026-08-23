package content

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/registry"
)

func TestExtractStaticHTMLWithConfiguredSelectors(t *testing.T) {
	document := `<!doctype html><html><body>
		<h1 class="headline">ප්‍රධාන පුවත</h1>
		<span class="author">කර්තෘ</span>
		<time class="date" datetime="2026-08-22T09:30:00+05:30"></time>
		<article class="story"><p>` + strings.Repeat("මෙය සම්පූර්ණ සිංහල ප්‍රවෘත්ති අන්තර්ගතයකි. ", 12) + `</p><div class="ad">දැන්වීම</div></article>
	</body></html>`
	config := registry.CollectionConfig{
		TitleSelector:        ".headline",
		AuthorSelector:       ".author",
		PublishedSelector:    ".date",
		ContentSelector:      ".story",
		ExcludeSelectors:     []string{".ad"},
		MinContentCharacters: 100,
		MinimumSinhalaRatio:  0.5,
	}

	result, err := extractStaticHTML([]byte(document), config)
	require.NoError(t, err)
	require.Equal(t, "ප්‍රධාන පුවත", result.Title)
	require.Equal(t, "කර්තෘ", result.Author)
	require.NotNil(t, result.PublishedAt)
	require.NotContains(t, result.BodyText, "දැන්වීම")
	require.Greater(t, result.Characters, 100)
	require.NoError(t, validateExtraction(result, config))
}

func TestExtractStaticHTMLUsesJSONLDArticleBody(t *testing.T) {
	document := `<html><head><script type="application/ld+json">{
		"@type":"NewsArticle",
		"headline":"සිංහල සිරස්තලය",
		"author":{"name":"වාර්තාකරු"},
		"datePublished":"2026-08-22T10:00:00Z",
		"articleBody":"` + strings.Repeat("සම්පූර්ණ පුවත් අන්තර්ගතය. ", 20) + `"
	}</script></head><body></body></html>`

	result, err := extractStaticHTML([]byte(document), registry.CollectionConfig{})
	require.NoError(t, err)
	require.Equal(t, "json_ld", result.Method)
	require.Equal(t, "සිංහල සිරස්තලය", result.Title)
	require.Equal(t, "වාර්තාකරු", result.Author)
	require.Greater(t, result.Characters, 100)
}

func TestExtractStructuredTextRemovesActiveMarkup(t *testing.T) {
	body, err := extractStructuredText(`<div><p>ආරක්ෂිත අන්තර්ගතය</p><script>alert(1)</script><style>body{}</style></div>`)

	require.NoError(t, err)
	require.Equal(t, "ආරක්ෂිත අන්තර්ගතය", body)
}

func TestURLPatterns(t *testing.T) {
	require.True(t, urlMatchesPatterns("https://news.example/story/42", []string{`^https://news\.example/story/`}))
	require.False(t, urlMatchesPatterns("https://news.example/category/42", []string{`/story/`}))
	require.True(t, urlMatchesPatterns("https://news.example/anything", nil))
}
