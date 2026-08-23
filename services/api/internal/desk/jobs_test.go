package desk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOverallWorkflowStatusQueued(t *testing.T) {
	status := overallWorkflowStatus("queued", []PipelineStep{{Status: "queued"}, {Status: "queued"}})

	require.Equal(t, "queued", status)
}

func TestOverallWorkflowStatusProcessing(t *testing.T) {
	status := overallWorkflowStatus("running", []PipelineStep{{Status: "succeeded"}, {Status: "running"}})

	require.Equal(t, "processing", status)
}

func TestOverallWorkflowStatusCompleted(t *testing.T) {
	status := overallWorkflowStatus("succeeded", []PipelineStep{{Status: "succeeded"}, {Status: "skipped"}})

	require.Equal(t, "completed", status)
}

func TestOverallWorkflowStatusPartiallyCompleted(t *testing.T) {
	status := overallWorkflowStatus("failed", []PipelineStep{{Status: "succeeded"}, {Status: "failed"}, {Status: "queued"}})

	require.Equal(t, "partially_completed", status)
}

func TestOverallWorkflowStatusFailed(t *testing.T) {
	status := overallWorkflowStatus("failed", []PipelineStep{{Status: "failed"}, {Status: "queued"}})

	require.Equal(t, "failed", status)
}

func TestCronJobHealthRunning(t *testing.T) {
	now := time.Date(2026, time.August, 22, 9, 0, 0, 0, time.UTC)
	latest := now.Add(-time.Minute)

	health := cronJobHealth(now, time.Minute, 1, &latest, "running", 0)

	require.Equal(t, "running", health)
}

func TestCronJobHealthHealthy(t *testing.T) {
	now := time.Date(2026, time.August, 22, 9, 0, 0, 0, time.UTC)
	latest := now.Add(-30 * time.Second)

	health := cronJobHealth(now, time.Minute, 0, &latest, "completed", 0)

	require.Equal(t, "healthy", health)
}

func TestCronJobHealthDegradedAfterFailure(t *testing.T) {
	now := time.Date(2026, time.August, 22, 9, 0, 0, 0, time.UTC)
	latest := now.Add(-30 * time.Second)

	health := cronJobHealth(now, time.Minute, 0, &latest, "completed", 1)

	require.Equal(t, "degraded", health)
}

func TestCronJobHealthFailed(t *testing.T) {
	now := time.Date(2026, time.August, 22, 9, 0, 0, 0, time.UTC)
	latest := now.Add(-30 * time.Second)

	health := cronJobHealth(now, time.Minute, 0, &latest, "discarded", 1)

	require.Equal(t, "failed", health)
}

func TestCronJobHealthOverdue(t *testing.T) {
	now := time.Date(2026, time.August, 22, 9, 0, 0, 0, time.UTC)
	latest := now.Add(-3 * time.Minute)

	health := cronJobHealth(now, time.Minute, 0, &latest, "completed", 0)

	require.Equal(t, "overdue", health)
}

func TestCronJobHealthUnknown(t *testing.T) {
	now := time.Date(2026, time.August, 22, 9, 0, 0, 0, time.UTC)

	health := cronJobHealth(now, time.Minute, 0, nil, "", 0)

	require.Equal(t, "unknown", health)
}

func TestCronScheduleAnchorUsesWorkerElectionAfterRestart(t *testing.T) {
	lastRun := time.Date(2026, time.August, 22, 8, 37, 0, 0, time.UTC)
	electedAt := time.Date(2026, time.August, 22, 10, 56, 0, 0, time.UTC)

	anchor := cronScheduleAnchor(&lastRun, &electedAt, false)

	require.NotNil(t, anchor)
	require.Equal(t, electedAt, *anchor)
}

func TestCronScheduleAnchorKeepsLatestRunForRunOnStartJob(t *testing.T) {
	lastRun := time.Date(2026, time.August, 22, 10, 56, 1, 0, time.UTC)
	electedAt := time.Date(2026, time.August, 22, 10, 56, 0, 0, time.UTC)

	anchor := cronScheduleAnchor(&lastRun, &electedAt, true)

	require.NotNil(t, anchor)
	require.Equal(t, lastRun, *anchor)
}
