package newsletter

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestStoreBuildDigestIntegration(t *testing.T) {
	databaseURL := os.Getenv("SNAP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SNAP_TEST_DATABASE_URL is not configured")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	location, err := time.LoadLocation("Asia/Colombo")
	require.NoError(t, err)
	windowEnd := time.Date(2026, 8, 27, 8, 0, 0, 0, location)
	digest, err := NewStore(pool).BuildDigest(
		context.Background(), "2026-08-27", windowEnd.AddDate(0, 0, -1), windowEnd,
		"https://lankanewspaper.prabhavalabs.com",
	)

	require.NoError(t, err)
	require.NotNil(t, digest.Stories)
	require.LessOrEqual(t, len(digest.Stories), 30)
}
