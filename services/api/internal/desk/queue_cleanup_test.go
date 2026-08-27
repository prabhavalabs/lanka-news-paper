package desk

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueueCleanupConfirmation(t *testing.T) {
	t.Parallel()

	failed, err := queueCleanupConfirmation(QueueCleanupFailed)
	require.NoError(t, err)
	require.Equal(t, "DELETE FAILED JOBS", failed)

	all, err := queueCleanupConfirmation(QueueCleanupAll)
	require.NoError(t, err)
	require.Equal(t, "DELETE QUEUE HISTORY", all)

	_, err = queueCleanupConfirmation("active")
	require.True(t, errors.Is(err, ErrInvalidQueueCleanupScope))
}

func TestQueueCleanupPredicatesProtectActiveJobs(t *testing.T) {
	t.Parallel()

	river, pipeline, err := queueCleanupPredicates(QueueCleanupAll)
	require.NoError(t, err)
	require.NotContains(t, river, "running")
	require.NotContains(t, river, "available")
	require.NotContains(t, river, "retryable")
	require.Equal(t, "status IN ('succeeded', 'failed')", pipeline)
}
