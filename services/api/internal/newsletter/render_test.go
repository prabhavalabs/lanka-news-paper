package newsletter

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRenderEditionIncludesSinhalaContentAndUnsubscribeLink(t *testing.T) {
	digest := Digest{
		EditionDate: "2026-08-27", ArticleCount: 12, EventCount: 4, SourceCount: 5,
		WindowStart: time.Date(2026, 8, 26, 2, 30, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 8, 27, 2, 30, 0, 0, time.UTC),
		Stories: []Story{{
			ID: "event-1", Kind: "event", Title: "ප්‍රධාන පුවත", Summary: "විස්තරාත්මක සාරාංශය.",
			Category: "දේශපාලන", URL: "https://news.example.com/e/event-1", ArticleCount: 3, SourceCount: 2,
		}},
	}

	rendered, err := RenderEdition(digest, "Nipun", "https://news.example.com/unsubscribe/token")

	require.NoError(t, err)
	require.Contains(t, rendered.HTML, "ප්‍රධාන පුවත")
	require.Contains(t, rendered.HTML, "Noto Sans Sinhala")
	require.Contains(t, rendered.HTML, "https://news.example.com/unsubscribe/token")
	require.Contains(t, rendered.Text, "ප්‍රධාන පුවත")
	require.True(t, strings.HasPrefix(rendered.Subject, "උදෑසන පුවත් සංග්‍රහය"))
}

func TestRenderEditionHandlesEmptyCoverage(t *testing.T) {
	digest := Digest{EditionDate: "2026-08-27", Stories: []Story{}}

	rendered, err := RenderEdition(digest, "", "https://news.example.com/unsubscribe/token")

	require.NoError(t, err)
	require.Contains(t, rendered.HTML, "ප්‍රකාශිත වැදගත් පුවත් හමු නොවීය")
	require.Contains(t, rendered.Text, "ප්‍රකාශිත වැදගත් පුවත් හමු නොවීය")
}
