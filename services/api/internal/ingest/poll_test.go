package ingest

import (
	"testing"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/require"
)

func TestPreferredItemContentUsesStructuredContent(t *testing.T) {
	item := &gofeed.Item{
		Content:     "<p>Complete structured article body.</p>",
		Description: "Short summary.",
	}

	content := preferredItemContent(item)

	require.Equal(t, item.Content, content)
}

func TestPreferredItemContentFallsBackToRSSDescription(t *testing.T) {
	item := &gofeed.Item{
		Description: "<p>Complete Blogger RSS article body.</p>",
	}

	content := preferredItemContent(item)

	require.Equal(t, item.Description, content)
}

func TestPreferredItemContentUsesCompleteBloggerRSSDescription(t *testing.T) {
	rss := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Gossip Lanka News</title>
    <link>https://www.gossiplankanews.com/</link>
    <description>Official Sinhala news feed</description>
    <item>
      <title>සම්පූර්ණ පුවත</title>
      <link>https://www.gossiplankanews.com/2026/08/example.html</link>
      <guid>gossip-example</guid>
      <description><![CDATA[<article><p>මෙය RSS විස්තර ක්ෂේත්‍රයෙන් ලැබෙන සම්පූර්ණ සිංහල පුවත් අන්තර්ගතයයි.</p><p>දෙවන ඡේදයද මෙහි අඩංගු වේ.</p></article>]]></description>
    </item>
  </channel>
</rss>`

	feed, err := ParseEndpoint("rss", "https://www.gossiplankanews.com", []byte(rss))

	require.NoError(t, err)
	require.Len(t, feed.Items, 1)
	require.Contains(t, preferredItemContent(feed.Items[0]), "දෙවන ඡේදය")
}
