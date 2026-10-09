package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/classify"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/politics"
	"github.com/stretchr/testify/require"
)

func analysisTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("SNAP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SNAP_TEST_DATABASE_URL is not configured")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	var name string
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT current_database()`).Scan(&name))
	require.Equal(t, "snap_test", name)
	return pool
}

func analysisTestArticle(t *testing.T, pool *pgxpool.Pool, article string) string {
	t.Helper()
	ctx := context.Background()
	sourceID, endpointID, rightsID, articleID, eventID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO sources(id,name,legal_name,source_type,website,active) VALUES($1,$2,$2,'other','https://pipeline.example.invalid',false)`, sourceID, "Combined analysis "+sourceID.String())
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO source_endpoints(id,source_id,endpoint_type,url,paused) VALUES($1,$2,'rss','https://pipeline.example.invalid/feed',true)`, endpointID, sourceID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO rights_profiles(id,source_id,endpoint_id,mode,attribution,effective_from) VALUES($1,$2,$3,'discovery_only','Combined analysis test',clock_timestamp())`, rightsID, sourceID, endpointID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO source_compliance_reviews(source_id,version,active,status,allow_discovery,allow_ai_processing,notes,reviewed_by) VALUES($1,1,true,'restricted',true,true,'Combined analysis test','test')`, sourceID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO event_clusters(id,display_title) VALUES($1,'Isolated analysis fixture')`, eventID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO articles(id,source_id,endpoint_id,rights_profile_id,source_item_id,original_url,canonical_url,headline,description,fingerprint,published_at,public_status,event_id) VALUES($1,$2,$3,$4,$5,'https://pipeline.example.invalid/story','https://pipeline.example.invalid/story','Community update',$6,$5,clock_timestamp(),'published',$7)`, articleID, sourceID, endpointID, rightsID, articleID.String(), article, eventID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	t.Cleanup(func() {
		for _, query := range []string{`DELETE FROM llm_calls WHERE article_id=$1`, `DELETE FROM articles WHERE id=$1`} {
			_, err := pool.Exec(ctx, query, articleID)
			require.NoError(t, err)
		}
		_, err := pool.Exec(ctx, `DELETE FROM event_clusters WHERE id=$1`, eventID)
		require.NoError(t, err)
		for _, query := range []string{`DELETE FROM source_compliance_reviews WHERE source_id=$1`, `DELETE FROM rights_profiles WHERE source_id=$1`, `DELETE FROM source_endpoints WHERE source_id=$1`, `DELETE FROM sources WHERE id=$1`} {
			_, err := pool.Exec(ctx, query, sourceID)
			require.NoError(t, err)
		}
	})
	return articleID.String()
}

type loggedCompleter struct {
	*countingCompleter
	pool *pgxpool.Pool
}

func (model loggedCompleter) record(ctx context.Context, request llm.Request, response llm.Response, err error) (llm.Response, error) {
	if err != nil {
		return response, err
	}
	outcome := "ok"
	if response.Provider == "none" {
		outcome = "fallback"
	}
	err = model.pool.QueryRow(ctx, `INSERT INTO llm_calls(task,provider_id,model,outcome,article_id,pipeline_run_id,pipeline_step_id)
		VALUES($1,'codex_cli','gpt-6-luna',$2,$3,$4,$5) RETURNING id`, request.Task, outcome, request.ArticleID, request.PipelineRunID, request.PipelineStepID).Scan(&response.CallID)
	return response, err
}

func (model loggedCompleter) Complete(ctx context.Context, request llm.Request) (llm.Response, error) {
	response, err := model.countingCompleter.Complete(ctx, request)
	return model.record(ctx, request, response, err)
}

func (model loggedCompleter) CompleteWithModel(ctx context.Context, request llm.Request, provider, name string) (llm.Response, error) {
	response, err := model.countingCompleter.CompleteWithModel(ctx, request, provider, name)
	return model.record(ctx, request, response, err)
}

func TestCombinedAnalysisFullRunIntegration(t *testing.T) {
	pool := analysisTestPool(t)
	ctx := context.Background()
	for _, test := range []struct {
		name, article, invalidPart string
		fail, pinned               bool
		wantCalls                  []string
	}{
		{"non economic", "A community gathering concluded peacefully with participants returning home.", "", false, false, []string{"article_analysis"}},
		{"economic policy", "The welfare allocation will support more families following the announced decision.", "", false, false, []string{"article_analysis"}},
		{"keyword category wins", "The budget allocation will support more families following the announced decision.", "", false, false, []string{"article_analysis"}},
		{"pinned model", "The welfare allocation will support more families following the announced decision.", "", false, true, []string{"article_analysis"}},
		{"failed combined economic", "The welfare allocation will support more families following the announced decision.", "", true, false, []string{"article_analysis", "content_cleaning", "article_summary", "classify", "narration_framing"}},
		{"failed combined non economic", "A community gathering concluded peacefully with participants returning home.", "", true, false, []string{"article_analysis", "content_cleaning", "article_summary", "classify"}},
		{"invalid summary", "The welfare allocation will support more families following the announced decision.", "summary", false, false, []string{"article_analysis", "article_summary"}},
		{"invalid category", "The welfare allocation will support more families following the announced decision.", "category", false, false, []string{"article_analysis", "classify"}},
		{"invalid narration", "The welfare allocation will support more families following the announced decision.", "narration", false, false, []string{"article_analysis", "narration_framing"}},
		{"invalid cleaned dependencies", "The welfare allocation will support more families following the announced decision.", "cleaned", false, false, []string{"article_analysis", "content_cleaning", "article_summary", "classify", "narration_framing"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			articleID := analysisTestArticle(t, pool, test.article)
			fake := &countingCompleter{article: test.article, combined: combinedFixture(test.article), fail: test.fail}
			if test.invalidPart != "" {
				var parts map[string]json.RawMessage
				require.NoError(t, json.Unmarshal([]byte(fake.combined), &parts))
				parts[test.invalidPart] = json.RawMessage(`null`)
				data, err := json.Marshal(parts)
				require.NoError(t, err)
				fake.combined = string(data)
			}
			model := loggedCompleter{countingCompleter: fake, pool: pool}
			store := NewStore(pool, model, nil, politics.NewStore(pool, model))
			var runID string
			var err error
			if test.pinned {
				runID, err = store.StartWithModel(ctx, articleID, "manual", "codex_cli", "gpt-6.1-sol")
			} else {
				runID, err = store.Start(ctx, articleID, "test")
			}
			require.NoError(t, err)
			require.NoError(t, store.Process(ctx, runID))
			require.Equal(t, test.wantCalls, fake.calls)
			var status, cleaned, summary, provider, category string
			var relevant bool
			var stepsSucceeded int
			var callID int64
			require.NoError(t, pool.QueryRow(ctx, `SELECT status,combined_analysis_call_id FROM article_pipeline_runs WHERE id=$1`, runID).Scan(&status, &callID))
			require.Equal(t, "succeeded", status)
			var task string
			require.NoError(t, pool.QueryRow(ctx, `SELECT task FROM llm_calls WHERE id=$1`, callID).Scan(&task))
			require.Equal(t, "article_analysis", task)
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM article_pipeline_steps WHERE run_id=$1 AND status='succeeded'`, runID).Scan(&stepsSucceeded))
			require.Equal(t, 6, stepsSucceeded)
			require.NoError(t, pool.QueryRow(ctx, `SELECT cleaned_text,summary_text FROM article_analysis_documents WHERE article_id=$1`, articleID).Scan(&cleaned, &summary))
			require.Equal(t, test.article, cleaned)
			require.Equal(t, test.article, summary)
			require.NoError(t, pool.QueryRow(ctx, `SELECT relevant,provider_id FROM article_political_analysis WHERE article_id=$1`, articleID).Scan(&relevant, &provider))
			require.Equal(t, strings.Contains(test.article, "welfare") || strings.Contains(test.article, "budget"), relevant)
			if !relevant {
				require.Equal(t, "signal-gate", provider)
			}
			require.NoError(t, pool.QueryRow(ctx, `SELECT c.slug FROM articles a JOIN categories c ON c.id=a.category_id WHERE a.id=$1`, articleID).Scan(&category))
			keyword := classify.From(nil, "Community update", test.article)
			if keyword.Confidence >= 0.55 {
				require.Equal(t, keyword.Slug, category, "keyword classification must take precedence")
			} else {
				require.Equal(t, "latest", category)
			}
			cached, err := NewStore(pool, model, nil, nil).loadCombinedAnalysis(ctx, runID)
			require.NoError(t, err)
			require.NotNil(t, cached, "cache must survive constructing another store")
			if test.pinned {
				require.Equal(t, []string{"codex_cli:gpt-6.1-sol"}, fake.pinned)
				require.Equal(t, "gpt-6.1-sol", cached.Model)
			}
			if test.fail || test.invalidPart == "cleaned" {
				var fallbackLogs int
				require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM article_pipeline_logs WHERE run_id=$1 AND event='combined_analysis_fallback' AND level='warning'`, runID).Scan(&fallbackLogs))
				require.Equal(t, 1, fallbackLogs)
			}
			var logs int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM article_pipeline_logs WHERE run_id=$1 AND event='reused_combined_analysis'`, runID).Scan(&logs))
			if len(test.wantCalls) == 1 {
				wantLogs := 3
				if keyword.Confidence >= 0.55 {
					wantLogs--
				}
				if relevant {
					wantLogs++
				}
				require.Equal(t, wantLogs, logs)
			}
			t.Logf("tasks=%v all six steps succeeded; combined_call_id=%d", fake.calls, callID)
		})
	}
}

func TestCombinedAnalysisManualAndResumeIntegration(t *testing.T) {
	pool := analysisTestPool(t)
	ctx := context.Background()
	article := "The welfare allocation will support more families following the announced decision."
	articleID := analysisTestArticle(t, pool, article)
	fake := &countingCompleter{article: article, combined: combinedFixture(article)}
	model := loggedCompleter{countingCompleter: fake, pool: pool}
	newStore := func() *Store { return NewStore(pool, model, nil, politics.NewStore(pool, model)) }
	store := newStore()
	for _, test := range []struct{ step, task string }{{"content_cleaning", "article_analysis"}, {"summarization", "article_summary"}, {"categorization", "classify"}, {"stance_evaluation", "narration_framing"}} {
		fake.calls = nil
		runID, err := store.Run(ctx, articleID, test.step)
		require.NoError(t, err)
		require.NoError(t, store.Process(ctx, runID))
		require.Equal(t, []string{test.task}, fake.calls, test.step)
		var skipped int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM article_pipeline_steps WHERE run_id=$1 AND status='skipped'`, runID).Scan(&skipped))
		require.Equal(t, 5, skipped)
	}
	// Resume after a partially invalid combined response's standalone summary
	// fails; the successful combined call is durable and is not repeated.
	fake.calls = nil
	fake.failSummary = true
	var parts map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(fake.combined), &parts))
	parts["summary"] = json.RawMessage(`null`)
	data, err := json.Marshal(parts)
	require.NoError(t, err)
	fake.combined = string(data)
	runID, err := store.Start(ctx, articleID, "test")
	require.NoError(t, err)
	require.ErrorContains(t, store.Process(ctx, runID), "interrupted summary")
	fake.failSummary = false
	require.NoError(t, newStore().Process(ctx, runID))
	require.Equal(t, []string{"article_analysis", "article_summary", "article_summary"}, fake.calls)
	// Retrying the cleaning step in that same run also reuses the durable cache.
	_, err = pool.Exec(ctx, `UPDATE article_pipeline_steps SET status='queued' WHERE run_id=$1 AND name='content_cleaning'`, runID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE article_pipeline_runs SET status='queued' WHERE id=$1`, runID)
	require.NoError(t, err)
	require.NoError(t, newStore().Process(ctx, runID))
	require.Equal(t, []string{"article_analysis", "article_summary", "article_summary"}, fake.calls)
}
