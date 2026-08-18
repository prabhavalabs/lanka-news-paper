package ingest

import (
	"testing"

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
			body:    `[{"id":42,"date_gmt":"2026-08-16T15:59:19","link":"https://publisher.example/story","title":{"rendered":"සිංහල පුවත &#8211; අද"},"excerpt":{"rendered":"<p>විස්තරය</p>"}}]`,
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
		})
	}
}
