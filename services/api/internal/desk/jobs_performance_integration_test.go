package desk

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/pagination"
	"github.com/stretchr/testify/require"
)

func TestQueueJobsDefaultViewCompletesWithinBudget(t *testing.T) {
	databaseURL := os.Getenv("SNAP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SNAP_TEST_DATABASE_URL is not configured")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	since := time.Now().UTC().Add(-7 * 24 * time.Hour)
	monitor, err := NewStore(pool).QueueJobs(
		ctx,
		pagination.Params{Page: 1, PerPage: 25},
		"", "", "article.pipeline", &since,
	)

	require.NoError(t, err)
	require.LessOrEqual(t, len(monitor.Items), 25)
	require.Equal(t, monitor.Summary.Total, monitor.Pagination.Total)
}

func TestCronJobsCompletesWithinBudget(t *testing.T) {
	databaseURL := os.Getenv("SNAP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SNAP_TEST_DATABASE_URL is not configured")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)
	monitor, err := NewStore(pool).CronJobs(ctx)

	require.NoError(t, err)
	require.NotEmpty(t, monitor.Items)
}
