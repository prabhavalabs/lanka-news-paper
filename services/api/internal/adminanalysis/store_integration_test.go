package adminanalysis

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/database"
	"github.com/stretchr/testify/require"
)

func TestStoreCreatesSingleArticleRunFromEligibleFullContent(t *testing.T) {
	databaseURL := os.Getenv("SNAP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SNAP_TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, databaseURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var articleID string
	err = pool.QueryRow(ctx, eligibleArticlesSQL+" ORDER BY article.published_at DESC LIMIT 1").Scan(&articleID)
	require.NoError(t, err)

	store := NewStore(pool)
	preview, err := store.Preview(ctx, CreateRequest{
		Scope: "article", Provider: "openrouter", Model: "test-model", ArticleID: articleID,
	})
	require.NoError(t, err)
	require.Equal(t, 1, preview.Articles)

	run, err := store.Create(ctx, CreateRequest{
		Scope: "article", Provider: "openrouter", Model: "test-model", ArticleID: articleID,
	}, "integration-test")
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupContext, `DELETE FROM admin_analysis_backfills WHERE id = $1`, run.ID)
	})
	require.Equal(t, 1, run.TotalArticles)
	require.Equal(t, 1, run.PendingArticles)
	require.Equal(t, "queued", run.Status)
}

func TestStorePausesResumesCancelsAndDeletesBackfill(t *testing.T) {
	databaseURL := os.Getenv("SNAP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SNAP_TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, databaseURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var articleID string
	err = pool.QueryRow(ctx, eligibleArticlesSQL+" ORDER BY article.published_at DESC LIMIT 1").Scan(&articleID)
	require.NoError(t, err)
	store := NewStore(pool)
	run, err := store.Create(ctx, CreateRequest{
		Scope: "article", Provider: "openrouter", Model: "test-model", ArticleID: articleID,
	}, "integration-test")
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupContext, `DELETE FROM admin_analysis_backfills WHERE id = $1`, run.ID)
	})

	require.NoError(t, store.MarkRunStarted(ctx, run.ID))
	require.NoError(t, store.MarkQueued(ctx, run.ID, articleID, 123))
	paused, err := store.Pause(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, "paused", paused.Run.Status)
	require.Equal(t, 1, paused.Run.PendingArticles)
	require.ErrorIs(t, store.MarkRunning(ctx, run.ID, articleID, 1), ErrRunInactive)

	resumed, err := store.Resume(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, "queued", resumed.Status)
	require.NoError(t, store.MarkRunStarted(ctx, run.ID))
	require.NoError(t, store.MarkQueued(ctx, run.ID, articleID, 124))

	stopped, err := store.Cancel(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", stopped.Run.Status)
	require.Equal(t, 1, stopped.Run.CancelledArticles)
	require.ErrorIs(t, store.MarkRunning(ctx, run.ID, articleID, 2), ErrRunInactive)

	require.NoError(t, store.Delete(ctx, run.ID))
	_, err = store.Get(ctx, run.ID)
	require.True(t, errors.Is(err, pgx.ErrNoRows))
}
