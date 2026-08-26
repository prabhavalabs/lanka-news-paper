package adminanalysis

import (
	"context"
	"os"
	"testing"
	"time"

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
	defer pool.Close()

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
