package llm

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm/codex"
	"github.com/stretchr/testify/require"
)

func TestArticleAnalysisProfileAndWorkflowIntegration(t *testing.T) {
	databaseURL := os.Getenv("SNAP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SNAP_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	var databaseName, oldStatus string
	var oldDetail *string
	var oldChecked *time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT current_database()`).Scan(&databaseName))
	require.Equal(t, "snap_test", databaseName)
	require.NoError(t, pool.QueryRow(ctx, `SELECT status,status_detail,checked_at FROM llm_providers WHERE id='codex_cli'`).Scan(&oldStatus, &oldDetail, &oldChecked))
	t.Cleanup(func() {
		_, err := pool.Exec(ctx, `UPDATE llm_providers SET status=$1,status_detail=$2,checked_at=$3 WHERE id='codex_cli'`, oldStatus, oldDetail, oldChecked)
		require.NoError(t, err)
	})
	_, err = pool.Exec(ctx, `UPDATE llm_providers SET status='unknown',checked_at=NULL WHERE id='codex_cli'`)
	require.NoError(t, err)
	client := &fakeCodexClient{result: codex.Result{Text: `{}`, InputTokens: 10, OutputTokens: 2}}
	gateway := NewGateway(pool, client)
	profiles, err := gateway.ListProfiles(ctx)
	require.NoError(t, err)
	found := false
	for _, profile := range profiles {
		if profile.Task == "article_analysis" {
			found = true
			require.Equal(t, "Article analysis", profile.Name)
		}
	}
	require.True(t, found)
	request := Request{Task: "article_analysis", System: "Preserve the source facts.", Input: `{"article":"fixture"}`, JSONSchema: map[string]any{"type": "object"}}
	response, err := gateway.Complete(ctx, request)
	require.NoError(t, err)
	require.Positive(t, response.CallID)
	t.Cleanup(func() {
		_, err := pool.Exec(ctx, `DELETE FROM llm_calls WHERE id=$1`, response.CallID)
		require.NoError(t, err)
	})
	require.Equal(t, "codex_cli", response.Provider)
	require.Equal(t, "gpt-6-luna", client.options.Model)
	require.Equal(t, "medium", client.options.ReasoningEffort)
	require.Equal(t, 240*time.Second, client.options.Timeout)
	require.Equal(t, request.JSONSchema, client.options.JSONSchema)
	require.Contains(t, client.options.System, request.System)
	require.Contains(t, client.options.System, "Careful newsroom editor who preserves facts, attribution, and the source language.")
	var task string
	require.NoError(t, pool.QueryRow(ctx, `SELECT task FROM llm_calls WHERE id=$1`, response.CallID).Scan(&task))
	require.Equal(t, "article_analysis", task)
}
