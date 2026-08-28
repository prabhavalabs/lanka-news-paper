package llm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModelSupportsTasksWhenEveryTaskIsCompatible(t *testing.T) {
	model := Model{CompatibleTasks: []string{"watch_tower_retrieval", "watch_tower_answer"}}

	compatible := modelSupportsTasks(model, watchTowerTasks)

	require.True(t, compatible)
}

func TestModelSupportsTasksRejectsMissingTask(t *testing.T) {
	model := Model{CompatibleTasks: []string{"watch_tower_answer"}}

	compatible := modelSupportsTasks(model, watchTowerTasks)

	require.False(t, compatible)
}
