package trends

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseGoogleTrends(t *testing.T) {
	feed := `<?xml version="1.0"?><rss xmlns:ht="https://trends.google.com/trending/rss"><channel>
	<item><title>namal rajapaksa</title><ht:approx_traffic>2000+</ht:approx_traffic>
	<ht:news_item><ht:news_item_title>Namal &amp; the Airbus case</ht:news_item_title></ht:news_item></item>
	<item><title> </title></item></channel></rss>`
	google, err := ParseGoogleTrends([]byte(feed))
	require.NoError(t, err)
	require.Equal(t, []GoogleTrend{{Query: "namal rajapaksa", Traffic: "2000+", News: []string{"Namal & the Airbus case"}}}, google)
}

func TestSearchesParsesGoogleTraffic(t *testing.T) {
	require.Equal(t, 2000.0, searches("2000+"))
	require.Equal(t, 10000.0, searches("10K+"))
	require.Equal(t, 1500000.0, searches("1.5M+"))
	require.Equal(t, 0.0, searches(""))
}

func TestOutlierRatioScalesForYoungVideos(t *testing.T) {
	require.InDelta(t, 3.0, outlierRatio(30_000, 30, 10_000), 0.01, "a day-old video at 3x the median")
	young := outlierRatio(5_000, 3, 10_000)
	require.Greater(t, young, 1.2, "5k views after 3 hours beats a 10k-median channel's early pace")
	require.Equal(t, 0.0, outlierRatio(5_000, 3, 0), "no baseline yet")
}

func TestStrongestVideosKeepsChartAndOutliers(t *testing.T) {
	rank := 4
	videos := strongestVideos([]VideoSignal{
		{ID: "normal", Outlier: 1.0, Views: 50_000},
		{ID: "viral", Outlier: 6.5, Views: 400_000},
		{ID: "chart", Outlier: 0.8, Views: 900_000, ChartRank: &rank},
		{ID: "ahead", Outlier: 1.6, Views: 20_000},
		{ID: "bulletin", Title: "🔴 LIVE | Siyatha News | 06.00 PM | 19 - 11 - 2026", Outlier: 28, Views: 8_000},
		{ID: "evening", Title: "Hiru News 6.55 PM", Outlier: 9, Views: 90_000},
	}, 10)
	ids := []string{}
	for _, v := range videos {
		ids = append(ids, v.ID)
	}
	require.Equal(t, []string{"viral", "chart", "ahead"}, ids)
}

func TestScoreTopicsResolvesEvidenceAndStatus(t *testing.T) {
	stories := []StorySignal{{Key: "s1", ID: "story-court", Score: 160, Sources: 8, Acceleration: 2}, {Key: "s2", ID: "story-solo", Score: 40, Sources: 1}}
	google := []GoogleTrend{{Key: "g1", Query: "namal", Traffic: "5000+"}}
	videos := []VideoSignal{{Key: "y1", ID: "v1", Outlier: 4, Views: 300_000}}
	research := []WebTopic{{Key: "w1", Momentum: "rising", Evidence: []Link{{URL: "https://example.lk"}}}, {Key: "w2", Momentum: "steady"}}
	topics := scoreTopics([]group{
		{TitleEN: "Court", Storyline: "s1", YouTube: []string{"y1"}, Google: []string{"g1"}, Web: []string{"w1"}, Suitable: true},
		{TitleEN: "Fire", Web: []string{"w2"}, Suitable: false},
		{TitleEN: "Viral clip", Web: []string{"w1", "w9"}, Suitable: true},
		{TitleEN: "Solo", Storyline: "s2", Suitable: true},
		{TitleEN: "Nothing", Suitable: true},
	}, stories, google, videos, research)
	require.Len(t, topics, 3, "a reused key and a topic with no signal are dropped")
	require.Equal(t, "Court", topics[0].TitleEN)
	require.Equal(t, 1, topics[0].Rank)
	require.Equal(t, "ready", topics[0].Status)
	require.Equal(t, "story-court", *topics[0].StorylineID)
	require.Equal(t, 45.0, topics[0].Components.Agreement, "four agreeing sources")
	statuses := map[string]string{}
	for _, topic := range topics {
		statuses[topic.TitleEN] = topic.Status
	}
	require.Equal(t, map[string]string{"Court": "ready", "Fire": "skip", "Solo": "watch"}, statuses, "single-outlet storylines wait on the watch list")
}
