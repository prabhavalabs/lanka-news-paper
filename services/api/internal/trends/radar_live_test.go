package trends

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm/codex"
	"github.com/stretchr/testify/require"
)

type codexModel struct{ client *codex.Client }

func (model codexModel) Complete(ctx context.Context, request llm.Request) (llm.Response, error) {
	result, err := model.client.Complete(ctx, codex.CallOptions{Model: "gpt-6.1-sol", ReasoningEffort: "medium", System: request.System,
		Input: request.Input, JSONSchema: request.JSONSchema, WebSearch: request.WebSearch, Timeout: 10 * time.Minute})
	return llm.Response{Text: result.Text}, err
}

// TestRadarLive collects real YouTube statistics, runs live web research and merges the signals
// (without storylines). It needs a scratch database, a YouTube key and a Codex login.
func TestRadarLive(t *testing.T) {
	databaseURL, key := os.Getenv("SNAP_TEST_DATABASE_URL"), os.Getenv("SNAP_YOUTUBE_API_KEY")
	if os.Getenv("SNAP_TREND_LIVE_TEST") != "1" || databaseURL == "" || key == "" {
		t.Skip("set SNAP_TREND_LIVE_TEST=1, SNAP_TEST_DATABASE_URL and SNAP_YOUTUBE_API_KEY")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	defer pool.Close()
	store := NewStore(pool)
	model := codexModel{codex.NewClient(nil)}
	radar := NewRadar(Config{Store: store, Model: model, YouTube: YouTube{Key: key}})

	require.NoError(t, radar.Collect(ctx))
	var stored int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM trend_youtube_videos`).Scan(&stored))
	t.Logf("stored %d videos", stored)
	require.Greater(t, stored, 100)

	videos, err := store.videoSignals(ctx, 48*time.Hour, 25)
	require.NoError(t, err)
	for i := range videos {
		videos[i].Key = "y" + string(rune('a'+i))
	}
	for _, v := range videos[:min(8, len(videos))] {
		t.Logf("video %.1fx  %7d views  %4.0fh  chart=%v  %s — %s", v.Outlier, v.Views, v.AgeHours, v.ChartRank, v.Channel, v.Title)
	}
	google, err := FetchGoogleTrends(ctx, radar.config.HTTP)
	require.NoError(t, err)
	for i := range google {
		google[i].Key = "g" + string(rune('a'+i))
	}
	started := time.Now()
	research, err := Research(ctx, model, time.Now())
	require.NoError(t, err)
	t.Logf("web research: %d topics in %s", len(research), time.Since(started).Round(time.Second))
	for i := range research {
		research[i].Key = "w" + string(rune('a'+i))
		t.Logf("web %-7s %s — %s (%d links)", research[i].Momentum, research[i].TitleEN, research[i].Why, len(research[i].Evidence))
	}
	groups, err := mergeSignals(ctx, model, nil, google, videos, research)
	require.NoError(t, err)
	topics := scoreTopics(groups, nil, google, videos, research)
	require.NotEmpty(t, topics)
	for _, topic := range topics {
		t.Logf("#%d %5.1f %-5s %-13s yt=%.0f search=%.0f web=%.0f agree=%.0f | %s | %s", topic.Rank, topic.Score, topic.Status, topic.Category,
			topic.Components.YouTube, topic.Components.Search, topic.Components.Web, topic.Components.Agreement, topic.TitleEN, topic.Reason)
	}

	id, err := store.startRun(ctx)
	require.NoError(t, err)
	require.NoError(t, store.finishRun(ctx, id, research, map[string]int{"youtube_videos": len(videos)}, topics))
	run, err := store.Latest(ctx)
	require.NoError(t, err)
	require.Equal(t, "ready", run.Status)
	require.Len(t, run.Topics, len(topics))
}
