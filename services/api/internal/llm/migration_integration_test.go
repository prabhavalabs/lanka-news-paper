package llm

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestCodexMigrationRoundTrip(t *testing.T) {
	if os.Getenv("SNAP_CODEX_MIGRATION_ROUNDTRIP") != "1" {
		t.Skip("explicit scratch database migration test is not enabled")
	}
	databaseURL := os.Getenv("SNAP_TEST_DATABASE_URL")
	require.NotEmpty(t, databaseURL)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	defer pool.Close()
	var databaseName string
	require.NoError(t, pool.QueryRow(ctx, `SELECT current_database()`).Scan(&databaseName))
	require.Equal(t, "snap_test", databaseName)
	path, err := filepath.Abs("../../migrations")
	require.NoError(t, err)
	runner, err := migrate.New("file://"+path, databaseURL)
	require.NoError(t, err)
	defer runner.Close()
	version, dirty, err := runner.Version()
	require.NoError(t, err)
	require.Equal(t, uint(52), version)
	require.False(t, dirty)
	require.NoError(t, runner.Steps(-1))
	version, dirty, err = runner.Version()
	require.NoError(t, err)
	require.Equal(t, uint(51), version)
	require.False(t, dirty)
	var oldProvider, kind, model string
	require.NoError(t, pool.QueryRow(ctx, `SELECT p.id,p.kind,t.model FROM llm_providers p JOIN llm_task_profiles t ON t.provider_id=p.id WHERE task='classify'`).Scan(&oldProvider, &kind, &model))
	require.Equal(t, "openai_api", kind)
	require.Equal(t, "deepseek/deepseek-v4-flash-0731", model)
	var columnCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_name='llm_task_profiles' AND column_name='reasoning_effort'`).Scan(&columnCount))
	require.Equal(t, 1, columnCount, "version 51 already contains this column")
	var historicCallID int64
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO llm_calls(task,provider_id,model,outcome) VALUES('classify',$1,$2,'ok') RETURNING id`, oldProvider, model).Scan(&historicCallID))
	defer pool.Exec(ctx, `DELETE FROM llm_calls WHERE id=$1`, historicCallID)
	require.NoError(t, runner.Up())
	version, dirty, err = runner.Version()
	require.NoError(t, err)
	require.Equal(t, uint(52), version)
	require.False(t, dirty)
	var preservedProvider string
	require.NoError(t, pool.QueryRow(ctx, `SELECT provider_id FROM llm_calls WHERE id=$1`, historicCallID).Scan(&preservedProvider))
	require.Equal(t, oldProvider, preservedProvider)
	for _, task := range orderedTasks {
		var provider, model, effort string
		var timeout int
		require.NoError(t, pool.QueryRow(ctx, `SELECT provider_id,model,reasoning_effort,timeout_seconds FROM llm_task_profiles WHERE task=$1`, task).Scan(&provider, &model, &effort, &timeout))
		require.Equal(t, "codex_cli", provider)
		expectedModel, expectedTimeout := "gpt-6-luna", 180
		if contains([]string{"newsletter_editorial", "watch_tower_answer", "admin_article_analysis"}, task) {
			expectedModel, expectedTimeout = "gpt-6.1-sol", 300
		}
		require.Equal(t, expectedModel, model)
		require.Equal(t, expectedTimeout, timeout)
		expectedEffort := "medium"
		if contains([]string{"classify", "content_cleaning", "watch_tower_retrieval"}, task) {
			expectedEffort = "low"
		}
		require.Equal(t, expectedEffort, effort)
	}
	t.Log("migration 52 -> 51 -> 52 clean; prior provider history preserved; all task defaults verified")
}
