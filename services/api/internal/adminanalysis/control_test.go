package adminanalysis

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPauseTransitionAcceptsQueuedRun(t *testing.T) {
	require.NoError(t, validateControlTransition("pause", "queued"))
}

func TestPauseTransitionAcceptsRunningRun(t *testing.T) {
	require.NoError(t, validateControlTransition("pause", "running"))
}

func TestPauseTransitionRejectsCompletedRun(t *testing.T) {
	err := validateControlTransition("pause", "completed")
	require.ErrorIs(t, err, ErrInvalidRunTransition)
}

func TestResumeTransitionOnlyAcceptsPausedRun(t *testing.T) {
	require.NoError(t, validateControlTransition("resume", "paused"))
	require.ErrorIs(t, validateControlTransition("resume", "running"), ErrInvalidRunTransition)
}

func TestCancelTransitionAcceptsEveryActiveRun(t *testing.T) {
	require.NoError(t, validateControlTransition("cancel", "queued"))
	require.NoError(t, validateControlTransition("cancel", "running"))
	require.NoError(t, validateControlTransition("cancel", "paused"))
}

func TestDeleteTransitionOnlyAcceptsTerminalRun(t *testing.T) {
	for _, status := range []string{"completed", "partially_completed", "failed", "cancelled"} {
		require.NoError(t, validateControlTransition("delete", status))
	}
	require.ErrorIs(t, validateControlTransition("delete", "paused"), ErrInvalidRunTransition)
}
