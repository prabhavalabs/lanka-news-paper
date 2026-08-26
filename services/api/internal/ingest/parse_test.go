package ingest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseRESTEndpoints(t *testing.T) {
	tests := []struct {
		name    string
		website string
		body    string
		link    string
		title   string
	}{
		{
			name:    "WordPress",
			website: "https://publisher.example",
			body:    `[{"id":42,"date_gmt":"2026-08-16T15:59:19","link":"https://publisher.example/story","title":{"rendered":"සිංහල පුවත &#8211; අද"},"excerpt":{"rendered":"<p>විස්තරය</p>"},"content":{"rendered":"<p>සම්පූර්ණ අන්තර්ගතය</p>"}}]`,
			link:    "https://publisher.example/story",
			title:   "සිංහල පුවත – අද",
		},
		{
			name:    "wrapped service",
			website: "https://news.example",
			body:    `{"latestPost":[{"id":"84","date_gmt":"2026-08-16T12:19:00.000Z","post_url":"2026/08/16/story","title":{"rendered":"තවත් සිංහල පුවත"}}]}`,
			link:    "https://news.example/2026/08/16/story",
			title:   "තවත් සිංහල පුවත",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			feed, err := ParseEndpoint("rest_api", test.website, []byte(test.body))
			require.NoError(t, err)
			require.Len(t, feed.Items, 1)
			require.Equal(t, test.link, feed.Items[0].Link)
			require.Equal(t, test.title, feed.Items[0].Title)
			require.NotNil(t, feed.Items[0].PublishedParsed)
			if test.name == "WordPress" {
				require.Contains(t, feed.Items[0].Content, "සම්පූර්ණ අන්තර්ගතය")
			}
		})
	}
}

func TestParseHiruRESTEndpoint(t *testing.T) {
	body := `[{"sinhala_title":"හිරු පුවත &#8211; අද","sinhala_story":"සම්පූර්ණ සිංහල පුවත් අන්තර්ගතය","sinhala_added_date":"2026-08-24 00:05:40","sinhala_art_id":"484729","seourltitle":"484729/example-story"}]`

	feed, err := ParseEndpoint("rest_api", "https://hirunews.lk", []byte(body))
	require.NoError(t, err)
	require.Len(t, feed.Items, 1)

	item := feed.Items[0]
	require.Equal(t, "484729", item.GUID)
	require.Equal(t, "හිරු පුවත – අද", item.Title)
	require.Equal(t, "https://hirunews.lk/484729/example-story", item.Link)
	require.Equal(t, "සම්පූර්ණ සිංහල පුවත් අන්තර්ගතය", item.Description)
	require.Equal(t, "සම්පූර්ණ සිංහල පුවත් අන්තර්ගතය", item.Content)
	require.NotNil(t, item.PublishedParsed)
	require.Equal(t, time.Date(2026, time.August, 23, 18, 35, 40, 0, time.UTC), item.PublishedParsed.UTC())
}
