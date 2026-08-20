package jobs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestQueueHistoryCutoff(t *testing.T) {
	now := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)

	require.Equal(t, now.Add(-7*24*time.Hour), queueHistoryCutoff(now))
}
