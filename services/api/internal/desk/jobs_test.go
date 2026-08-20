package desk

import (
	"testing"

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
