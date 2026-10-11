package retention

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestRunIntegration(t *testing.T) {
	databaseURL := os.Getenv("SNAP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SNAP_TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	now := time.Now().UTC()
	old, recent := now.Add(-10*24*time.Hour), now.Add(-24*time.Hour)
	sourceID, endpointID, rightsID, articleID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM articles WHERE id = $1`, articleID)
		_, _ = pool.Exec(cleanup, `DELETE FROM ingestion_runs WHERE endpoint_id = $1`, endpointID)
		_, _ = pool.Exec(cleanup, `DELETE FROM rights_profiles WHERE id = $1`, rightsID)
		_, _ = pool.Exec(cleanup, `DELETE FROM source_endpoints WHERE id = $1`, endpointID)
		_, _ = pool.Exec(cleanup, `DELETE FROM sources WHERE id = $1`, sourceID)
	})
	exec := func(query string, arguments ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, query, arguments...)
		require.NoError(t, err)
	}
	exec(`INSERT INTO sources (id, name, legal_name, source_type, website, active) VALUES ($1, $2, $2, 'other', 'https://retention.example', false)`, sourceID, "Retention "+sourceID.String())
	exec(`INSERT INTO source_endpoints (id, source_id, endpoint_type, url, paused) VALUES ($1, $2, 'rss', 'https://retention.example/feed.xml', true)`, endpointID, sourceID)
	exec(`INSERT INTO rights_profiles (id, source_id, endpoint_id, mode, attribution, effective_from, raw_payload_retention_days)
		VALUES ($1, $2, $3, 'full_syndication', 'Test', clock_timestamp(), 7)`, rightsID, sourceID, endpointID)
	exec(`INSERT INTO articles (id, source_id, endpoint_id, rights_profile_id, source_item_id, original_url, canonical_url, headline, fingerprint, published_at)
		VALUES ($1, $2, $3, $4, $5, 'https://retention.example/1', 'https://retention.example/1', 'පරීක්ෂණ', $5, clock_timestamp())`,
		articleID, sourceID, endpointID, rightsID, articleID.String())

	var oldRun, recentRun uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO ingestion_runs (endpoint_id, started_at, status) VALUES ($1, $2, 'ok') RETURNING id`, endpointID, old).Scan(&oldRun))
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO ingestion_runs (endpoint_id, started_at, status) VALUES ($1, $2, 'ok') RETURNING id`, endpointID, recent).Scan(&recentRun))
	var oldAttempt, keptFailure, recentAttempt uuid.UUID
	attempt := `INSERT INTO crawl_attempts (article_id, source_id, requested_url, status, started_at) VALUES ($1, $2, 'https://retention.example/1', $3, $4) RETURNING id`
	require.NoError(t, pool.QueryRow(ctx, attempt, articleID, sourceID, "skipped", old).Scan(&oldAttempt))
	require.NoError(t, pool.QueryRow(ctx, attempt, articleID, sourceID, "failed", old).Scan(&keptFailure))
	require.NoError(t, pool.QueryRow(ctx, attempt, articleID, sourceID, "skipped", recent).Scan(&recentAttempt))
	var oldCall, recentCall int64
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO llm_calls (task, outcome, created_at) VALUES ('retention_test', 'ok', $1) RETURNING id`, old).Scan(&oldCall))
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO llm_calls (task, outcome, created_at) VALUES ('retention_test', 'ok', $1) RETURNING id`, recent).Scan(&recentCall))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM llm_calls WHERE id = $1`, recentCall) })

	directory := t.TempDir()
	oldFile, recentFile := filepath.Join(directory, "sessions", "old.jsonl"), filepath.Join(directory, "recent.log")
	require.NoError(t, os.MkdirAll(filepath.Dir(oldFile), 0o755))
	require.NoError(t, os.WriteFile(oldFile, []byte("old"), 0o600))
	require.NoError(t, os.WriteFile(recentFile, []byte("recent"), 0o600))
	require.NoError(t, os.Chtimes(oldFile, old, old))

	store := NewStore(pool, nil, directory, filepath.Join(directory, "missing"))
	_, err = store.UpdateSettings(ctx, uuid.Nil, 0)
	require.ErrorIs(t, err, ErrInvalid)
	_, err = store.UpdateSettings(ctx, uuid.Nil, DefaultDays)
	require.NoError(t, err)

	summary, err := store.Run(ctx, now)
	require.NoError(t, err)
	require.GreaterOrEqual(t, summary["ingestion_runs"], int64(1))
	require.GreaterOrEqual(t, summary["crawl_attempts"], int64(1))
	require.GreaterOrEqual(t, summary["model_calls"], int64(1))
	require.Equal(t, int64(1), summary["log_files"])

	exists := func(query string, id any) bool {
		var found bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS (`+query+`)`, id).Scan(&found))
		return found
	}
	require.False(t, exists(`SELECT 1 FROM ingestion_runs WHERE id = $1`, oldRun))
	require.True(t, exists(`SELECT 1 FROM ingestion_runs WHERE id = $1`, recentRun))
	require.False(t, exists(`SELECT 1 FROM crawl_attempts WHERE id = $1`, oldAttempt))
	require.True(t, exists(`SELECT 1 FROM crawl_attempts WHERE id = $1`, keptFailure), "failed fetch of an article without a body must stay")
	require.True(t, exists(`SELECT 1 FROM crawl_attempts WHERE id = $1`, recentAttempt))
	require.False(t, exists(`SELECT 1 FROM llm_calls WHERE id = $1`, oldCall))
	require.True(t, exists(`SELECT 1 FROM llm_calls WHERE id = $1`, recentCall))
	require.NoFileExists(t, oldFile)
	require.FileExists(t, recentFile)

	settings, err := store.Settings(ctx)
	require.NoError(t, err)
	require.Equal(t, DefaultDays, settings.RetentionDays)
	require.NotNil(t, settings.LastRunAt)
	require.Nil(t, settings.LastRunError)
	require.Equal(t, int64(1), settings.LastRunSummary["log_files"])
}
