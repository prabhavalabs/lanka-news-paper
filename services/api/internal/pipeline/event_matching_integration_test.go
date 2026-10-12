package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/cluster"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/stretchr/testify/require"
)

// matchingCompleter answers event matching with the event whose headline holds want.
type matchingCompleter struct {
	want  string
	calls int
}

func (model *matchingCompleter) Complete(_ context.Context, request llm.Request) (llm.Response, error) {
	if request.Task != "event_matching" {
		return llm.Response{}, fmt.Errorf("unexpected task %s", request.Task)
	}
	model.calls++
	var input struct {
		Events []struct {
			Number    int      `json:"number"`
			Headlines []string `json:"headlines"`
		} `json:"events"`
	}
	if err := json.Unmarshal([]byte(request.Input), &input); err != nil {
		return llm.Response{}, err
	}
	match := 0
	for _, event := range input.Events {
		if strings.Contains(strings.Join(event.Headlines, " "), model.want) {
			match = event.Number
		}
	}
	return llm.Response{Provider: "codex_cli", Text: fmt.Sprintf(`{"match":%d,"reason":"Same shipwreck."}`, match)}, nil
}

func (model *matchingCompleter) CompleteWithModel(ctx context.Context, request llm.Request, _, _ string) (llm.Response, error) {
	return model.Complete(ctx, request)
}

func TestEventMatchingJoinsSameStoryIntegration(t *testing.T) {
	pool := analysisTestPool(t)
	ctx := context.Background()
	first := analysisTestArticle(t, pool, "A migrant boat capsized off Djibouti; 109 people died.")
	second := analysisTestArticle(t, pool, "A migrant boat capsized off Djibouti; 139 people died.")
	marker := first[:8]
	for id, headline := range map[string]string{
		first:  "Migrant boat capsizes off Djibouti " + marker + ", 109 dead and 116 missing",
		second: "Djibouti migrant vessel overturns " + marker + ", 139 killed",
	} {
		_, err := pool.Exec(ctx, `UPDATE articles SET headline=$2 WHERE id=$1`, id, headline)
		require.NoError(t, err)
	}
	eventOf := func(id string) string {
		var event string
		require.NoError(t, pool.QueryRow(ctx, `SELECT event_id::text FROM articles WHERE id=$1`, id).Scan(&event))
		return event
	}
	firstEvent, secondEvent := eventOf(first), eventOf(second)
	model := &matchingCompleter{want: marker + ", 109"}
	store := NewStore(pool, model, cluster.NewStore(pool), nil)

	// The earlier report is never offered the later one, so the two cannot join both ways.
	result, err := store.cluster(ctx, first, "", "", modelSelection{})
	require.NoError(t, err)
	require.Equal(t, firstEvent, eventOf(first))

	result, err = store.cluster(ctx, second, "", "", modelSelection{})
	require.NoError(t, err)
	require.Equal(t, firstEvent, eventOf(second), "the later report joins the earlier event")
	require.Equal(t, firstEvent, result.output.(map[string]any)["event_match"])
	var status string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM event_clusters WHERE id=$1`, secondEvent).Scan(&status))
	require.Equal(t, "merged", status)
	var members int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM event_articles WHERE event_id=$1 AND article_id=$2`, firstEvent, second).Scan(&members))
	require.Equal(t, 1, members)

	// An event with more than one report is settled and is not matched again.
	calls := model.calls
	_, err = store.cluster(ctx, second, "", "", modelSelection{})
	require.NoError(t, err)
	require.Equal(t, calls, model.calls)
	_, err = pool.Exec(ctx, `UPDATE articles SET event_id=$2 WHERE id=$1`, second, secondEvent)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM event_articles WHERE article_id=$1`, second)
	require.NoError(t, err)
}

func TestEventMatchingKeepsDifferentStoriesApartIntegration(t *testing.T) {
	pool := analysisTestPool(t)
	ctx := context.Background()
	first := analysisTestArticle(t, pool, "A bus crashed in Badulla.")
	second := analysisTestArticle(t, pool, "A bus crashed in Kandy.")
	marker := first[:8]
	for id, headline := range map[string]string{first: "Bus crash in Badulla " + marker, second: "Bus crash in Kandy " + marker} {
		_, err := pool.Exec(ctx, `UPDATE articles SET headline=$2 WHERE id=$1`, id, headline)
		require.NoError(t, err)
	}
	var before string
	require.NoError(t, pool.QueryRow(ctx, `SELECT event_id::text FROM articles WHERE id=$1`, second).Scan(&before))
	model := &matchingCompleter{want: "no such headline"}
	store := NewStore(pool, model, cluster.NewStore(pool), nil)
	result, err := store.cluster(ctx, second, "", "", modelSelection{})
	require.NoError(t, err)
	require.Equal(t, 1, model.calls)
	require.Equal(t, "none", result.output.(map[string]any)["event_match"])
	require.Equal(t, before, *result.output.(map[string]any)["event_id"].(*string))
}
