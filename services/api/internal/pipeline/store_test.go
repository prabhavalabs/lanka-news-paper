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

func TestAdministrativeOpenRouterRunUsesConfiguredProviderSort(t *testing.T) {
	store := NewStore(nil, nil, nil, nil, WithAdminProviderSort("throughput"))
	selection := store.routeSelection("admin_backfill", modelSelection{
		provider: "openrouter",
		model:    "deepseek/deepseek-v4-flash-0731",
	})

	require.Equal(t, "throughput", selection.providerSort)
}

func TestRegularPipelineRunDoesNotOverrideProviderRouting(t *testing.T) {
	store := NewStore(nil, nil, nil, nil, WithAdminProviderSort("throughput"))
	selection := store.routeSelection("ingestion", modelSelection{provider: "openrouter"})

	require.Empty(t, selection.providerSort)
}
