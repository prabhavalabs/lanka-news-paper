package jobs

import (
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/require"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/adminanalysis"
)

func TestQueueHistoryCutoff(t *testing.T) {
	now := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)

	require.Equal(t, now.Add(-7*24*time.Hour), queueHistoryCutoff(now))
}

func TestPeriodicJobCatalog(t *testing.T) {
	catalog := PeriodicJobCatalog()

	require.Len(t, catalog, 5)
	require.Equal(t, "ingest.poll", catalog[0].Kind)
	require.Equal(t, time.Minute, catalog[0].Interval)
	require.True(t, catalog[0].RunOnStart)
	require.Equal(t, "article.pipeline.dispatch", catalog[1].Kind)
	require.Equal(t, time.Minute, catalog[1].Interval)
	require.True(t, catalog[1].RunOnStart)
	require.Equal(t, "brief.daily", catalog[2].Kind)
	require.Equal(t, time.Hour, catalog[2].Interval)
	require.False(t, catalog[2].RunOnStart)
	require.Equal(t, "queue.history.cleanup", catalog[3].Kind)
	require.Equal(t, 24*time.Hour, catalog[3].Interval)
	require.True(t, catalog[3].RunOnStart)
	require.Equal(t, "article.content.cleanup", catalog[4].Kind)
	require.Equal(t, 24*time.Hour, catalog[4].Interval)
	require.True(t, catalog[4].RunOnStart)
}

func TestQueueCatalog(t *testing.T) {
	t.Setenv("SNAP_ADMIN_ANALYSIS_WORKERS", "")
	catalog := QueueCatalog()

	require.Len(t, catalog, 5)
	require.Equal(t, "default", catalog[0].Name)
	require.Equal(t, 2, catalog[0].MaxWorkers)
	require.Equal(t, "analysis", catalog[1].Name)
	require.Equal(t, 5, catalog[1].MaxWorkers)
	require.Equal(t, "crawl", catalog[2].Name)
	require.Equal(t, 1, catalog[2].MaxWorkers)
	require.Equal(t, "admin-analysis-dispatch", catalog[3].Name)
	require.Equal(t, 1, catalog[3].MaxWorkers)
	require.Equal(t, "admin-analysis", catalog[4].Name)
	require.Equal(t, 1, catalog[4].MaxWorkers)
}

func TestQueueCatalogUsesConfiguredAdministrativeConcurrency(t *testing.T) {
	t.Setenv("SNAP_ADMIN_ANALYSIS_WORKERS", "48")

	catalog := QueueCatalog()

	require.Equal(t, "admin-analysis", catalog[4].Name)
	require.Equal(t, 48, catalog[4].MaxWorkers)
}

func TestQueueCatalogBoundsAdministrativeConcurrency(t *testing.T) {
	t.Setenv("SNAP_ADMIN_ANALYSIS_WORKERS", "999")

	catalog := QueueCatalog()

	require.Equal(t, 64, catalog[4].MaxWorkers)
}

func TestAdminAnalysisJobsSeparateDispatchFromArticleExecution(t *testing.T) {
	dispatch := (AdminAnalysisBackfillDispatchArgs{RunID: "run"}).InsertOpts()
	article := (AdminArticleAnalysisArgs{RunID: "run", ArticleID: "article"}).InsertOpts()

	require.Equal(t, "admin-analysis-dispatch", dispatch.Queue)
	require.Equal(t, "admin-analysis", article.Queue)
	require.Equal(t, 4, article.Priority)
	require.True(t, dispatch.UniqueOpts.ByArgs)
	require.True(t, article.UniqueOpts.ByArgs)
}

func TestMissingCodexFailsAdministrativeAnalysisWithoutRetryBackoff(t *testing.T) {
	err := errors.Join(errors.New("clean article"), adminanalysis.ErrCodexUnavailable)

	require.True(t, adminAnalysisFailureIsTerminal(err, 1, 5))
	require.False(t, adminAnalysisFailureIsTerminal(errors.New("temporary provider error"), 1, 5))
	require.True(t, adminAnalysisFailureIsTerminal(errors.New("last attempt"), 5, 5))
}

func TestArticleContentBackfillIsUnique(t *testing.T) {
	opts := (ArticleContentBackfillArgs{}).InsertOpts()
	require.True(t, opts.UniqueOpts.ByQueue)
	require.Contains(t, opts.UniqueOpts.ByState, rivertype.JobStateRunning)
}

func TestArticleContentBackfillUsesBoundedLowPriorityBatches(t *testing.T) {
	require.Equal(t, 25, articleContentBackfillBatchSize)
	require.Equal(t, 4, articleContentBackfillInsertOpts().Priority)
}
