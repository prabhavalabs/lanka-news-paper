package llm

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm/codex"
	"github.com/stretchr/testify/require"
)

type gatewayRunner struct {
	executions int
	processes  int
	failure    string
	input      string
}

func (*gatewayRunner) LookPath(string) (string, error) { return "/fake/codex", nil }
func (runner *gatewayRunner) Run(_ context.Context, _ string, args []string, input, _ string) (string, string, error) {
	runner.processes++
	if args[0] == "login" {
		return "Logged in using ChatGPT", "", nil
	}
	if args[0] == "--version" {
		return "codex-cli 0.162.0", "", nil
	}
	runner.executions++
	runner.input = input
	if runner.failure != "" {
		return fmt.Sprintf(`{"type":"turn.failed","error":{"message":%q}}`, runner.failure), "", fmt.Errorf("exit status 1")
	}
	return "{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"politics\"}}\n{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":21,\"cached_input_tokens\":8,\"output_tokens\":3,\"reasoning_output_tokens\":2}}\n", "", nil
}

func TestGatewayCodexLoggingAndCooldownIntegration(t *testing.T) {
	databaseURL := os.Getenv("SNAP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SNAP_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	var databaseName string
	require.NoError(t, pool.QueryRow(ctx, `SELECT current_database()`).Scan(&databaseName))
	require.Equal(t, "snap_test", databaseName)
	var oldStatus string
	var oldDetail *string
	var oldChecked *time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT status,status_detail,checked_at FROM llm_providers WHERE id='codex_cli'`).Scan(&oldStatus, &oldDetail, &oldChecked))
	t.Cleanup(func() {
		_, err := pool.Exec(ctx, `UPDATE llm_providers SET status=$1,status_detail=$2,checked_at=$3 WHERE id='codex_cli'`, oldStatus, oldDetail, oldChecked)
		require.NoError(t, err)
	})
	_, err = pool.Exec(ctx, `UPDATE llm_providers SET status='unknown',checked_at=NULL WHERE id='codex_cli'`)
	require.NoError(t, err)
	sourceID, endpointID, rightsID, articleID, runID, stepID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO sources(id,name,legal_name,source_type,website,active) VALUES($1,$2,$2,'other','https://llm.example.invalid',false)`, sourceID, "LLM integration "+sourceID.String())
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO source_endpoints(id,source_id,endpoint_type,url,paused) VALUES($1,$2,'rss','https://llm.example.invalid/feed',true)`, endpointID, sourceID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO rights_profiles(id,source_id,endpoint_id,mode,attribution,effective_from) VALUES($1,$2,$3,'discovery_only','LLM test',clock_timestamp())`, rightsID, sourceID, endpointID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO articles(id,source_id,endpoint_id,rights_profile_id,source_item_id,original_url,canonical_url,headline,fingerprint,published_at) VALUES($1,$2,$3,$4,$5,'https://llm.example.invalid/story','https://llm.example.invalid/story','Parliament debates electoral reforms',$5,clock_timestamp())`, articleID, sourceID, endpointID, rightsID, articleID.String())
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO article_pipeline_runs(id,article_id,trigger) VALUES($1,$2,'test')`, runID, articleID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO article_pipeline_steps(id,run_id,name,position) VALUES($1,$2,'categorization',1)`, stepID, runID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	t.Cleanup(func() {
		for _, query := range []string{`DELETE FROM llm_calls WHERE article_id=$1`, `DELETE FROM articles WHERE id=$1`} {
			_, err := pool.Exec(ctx, query, articleID)
			require.NoError(t, err)
		}
		for _, query := range []string{`DELETE FROM rights_profiles WHERE source_id=$1`, `DELETE FROM source_endpoints WHERE source_id=$1`, `DELETE FROM sources WHERE id=$1`} {
			_, err := pool.Exec(ctx, query, sourceID)
			require.NoError(t, err)
		}
	})
	runner := &gatewayRunner{}
	gateway := NewGateway(pool, codex.NewClient(runner))
	request := Request{Task: "classify", System: "Classify input.", Input: "Parliament debates electoral reforms.", ArticleID: articleID.String(), PipelineRunID: runID.String(), PipelineStepID: stepID.String()}
	response, err := gateway.Complete(ctx, request)
	require.NoError(t, err)
	require.Equal(t, "codex_cli", response.Provider)
	require.Equal(t, "politics", response.Text)
	require.Equal(t, request.Input, runner.input)
	var outcome string
	var streamed bool
	var inputTokens, outputTokens int
	require.NoError(t, pool.QueryRow(ctx, `SELECT outcome,streamed,input_tokens,output_tokens FROM llm_calls WHERE pipeline_step_id=$1 ORDER BY id DESC LIMIT 1`, stepID).Scan(&outcome, &streamed, &inputTokens, &outputTokens))
	require.Equal(t, "ok", outcome)
	require.False(t, streamed)
	require.Equal(t, 21, inputTokens)
	require.Equal(t, 3, outputTokens)
	for _, test := range []struct {
		failure, status string
		kind            error
	}{{"You've hit your usage limit", "quota_exhausted", codex.ErrQuota}, {"401 Unauthorized", "auth_expired", codex.ErrAuth}} {
		runner.failure = test.failure
		_, err = gateway.CompleteWithModel(ctx, request, "codex_cli", "gpt-6-luna")
		require.ErrorIs(t, err, test.kind)
		var status string
		var checkedAt time.Time
		require.NoError(t, pool.QueryRow(ctx, `SELECT status,checked_at FROM llm_providers WHERE id='codex_cli'`).Scan(&status, &checkedAt))
		require.Equal(t, test.status, status)
		blocked, err := gateway.ProviderBlocked(ctx)
		require.NoError(t, err)
		require.True(t, blocked)
		count, processes := runner.executions, runner.processes
		response, err = gateway.Complete(ctx, request)
		require.NoError(t, err)
		require.Equal(t, "keyword-rules", response.Provider)
		response, err = gateway.Complete(ctx, Request{Task: "article_summary"})
		require.NoError(t, err)
		require.Equal(t, "none", response.Provider)
		_, err = gateway.CompleteWithModel(ctx, request, "codex_cli", "gpt-6-luna")
		require.ErrorIs(t, err, ErrProviderBlocked)
		require.Equal(t, count, runner.executions)
		require.Equal(t, processes, runner.processes, "blocked calls must not spawn any process")
		provider, err := gateway.ProviderStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, test.status, provider.Status)
		require.False(t, provider.Ready)
		var currentCheckedAt time.Time
		require.NoError(t, pool.QueryRow(ctx, `SELECT checked_at FROM llm_providers WHERE id='codex_cli'`).Scan(&currentCheckedAt))
		require.True(t, checkedAt.Equal(currentCheckedAt), "fast failures and probes must not extend cooldown")
		_, err = pool.Exec(ctx, `UPDATE llm_providers SET checked_at=clock_timestamp()-interval '16 minutes' WHERE id='codex_cli'`)
		require.NoError(t, err)
		runner.failure = ""
		response, err = gateway.Complete(ctx, request)
		require.NoError(t, err)
		require.Equal(t, "codex_cli", response.Provider)
		require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM llm_providers WHERE id='codex_cli'`).Scan(&status))
		require.Equal(t, "operational", status)
	}
	rows, err := pool.Query(ctx, `SELECT event FROM article_pipeline_logs WHERE run_id=$1 ORDER BY created_at`, runID)
	require.NoError(t, err)
	defer rows.Close()
	var events []string
	for rows.Next() {
		var event string
		require.NoError(t, rows.Scan(&event))
		events = append(events, event)
	}
	require.NoError(t, rows.Err())
	for _, event := range []string{"provider_request_started", "provider_response_completed", "provider_request_failed"} {
		require.Contains(t, events, event)
	}
	require.NotContains(t, strings.Join(events, ","), "provider_first_token")
}
