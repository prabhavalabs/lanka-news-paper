package pipeline

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompletionSlug(t *testing.T) {
	slug, valid := completionSlug("```Politics```\n")
	require.True(t, valid)
	require.Equal(t, "politics", slug)

	_, valid = completionSlug("The category is politics")
	require.False(t, valid)
}

func TestValidStep(t *testing.T) {
	require.True(t, validStep(""))
	require.True(t, validStep("event_clustering"))
	require.False(t, validStep("source_intake"))
}
