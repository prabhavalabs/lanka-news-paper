package social

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestSocialMigrationRoundTripIntegration(t *testing.T) {
	databaseURL := os.Getenv("SNAP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SNAP_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	root, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	schema := "social_migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = root.Exec(ctx, `CREATE SCHEMA `+schema)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = root.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`); root.Close() })
	for _, table := range []string{"admin_users", "articles", "event_clusters", "llm_calls", "llm_task_profiles", "agent_workflows", "agent_workflow_versions"} {
		_, err = root.Exec(ctx, `CREATE TABLE `+schema+`.`+table+` (LIKE public.`+table+` INCLUDING ALL)`)
		require.NoError(t, err)
	}
	_, err = root.Exec(ctx, `ALTER TABLE `+schema+`.agent_workflow_versions ADD FOREIGN KEY (task) REFERENCES `+schema+`.agent_workflows(task) ON DELETE CASCADE`)
	require.NoError(t, err)
	configuration, err := pgxpool.ParseConfig(databaseURL)
	require.NoError(t, err)
	configuration.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, configuration)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	up, err := os.ReadFile("../../migrations/000054_social_publishing.up.sql")
	require.NoError(t, err)
	down, err := os.ReadFile("../../migrations/000054_social_publishing.down.sql")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(up))
	require.NoError(t, err)
	var provider, model, effort string
	var timeout int
	err = pool.QueryRow(ctx, `SELECT provider_id,model,reasoning_effort,timeout_seconds FROM llm_task_profiles WHERE task='social_post_editorial'`).Scan(&provider, &model, &effort, &timeout)
	require.NoError(t, err)
	require.Equal(t, "codex_cli", provider)
	require.Equal(t, "gpt-6.1-sol", model)
	require.Equal(t, "medium", effort)
	require.Equal(t, 300, timeout)
	var enabled, auto bool
	err = pool.QueryRow(ctx, `SELECT enabled,auto_publish FROM social_settings`).Scan(&enabled, &auto)
	require.NoError(t, err)
	require.False(t, enabled)
	require.False(t, auto)
	_, err = pool.Exec(ctx, `UPDATE social_settings SET min_score=101`)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO social_accounts (page_id,page_name,status,token_ciphertext,token_nonce,key_version) VALUES ('one','One','connected','a','b','v1')`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO social_accounts (page_id,page_name,status,token_ciphertext,token_nonce,key_version) VALUES ('two','Two','connected','a','b','v1')`)
	require.Error(t, err)
	safetyUp, err := os.ReadFile("../../migrations/000055_social_publishing_safety.up.sql")
	require.NoError(t, err)
	safetyDown, err := os.ReadFile("../../migrations/000055_social_publishing_safety.down.sql")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(safetyUp))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE social_accounts SET publish_retry_at=now(); SELECT evidence_article_ids FROM social_posts LIMIT 1`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(safetyDown))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.NoError(t, err)
	var count int
	err = pool.QueryRow(ctx, `SELECT count(*)::int FROM agent_workflow_versions WHERE task='social_post_editorial'`).Scan(&count)
	require.NoError(t, err)
	require.Zero(t, count)
	err = pool.QueryRow(ctx, `SELECT count(*)::int FROM information_schema.tables WHERE table_schema=$1 AND table_name LIKE 'social_%'`, schema).Scan(&count)
	require.NoError(t, err)
	require.Zero(t, count)
	_, err = pool.Exec(ctx, string(up))
	require.NoError(t, err)
}
