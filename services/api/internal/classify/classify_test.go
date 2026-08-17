package classify

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFromCrossValidatesPublisherAndContent(t *testing.T) {
	result := From([]string{"ක්‍රීඩා පුවත් | Sports News"}, "සොනාල්ගේ මංගල ටෙස්ට් ශතකය", "")
	require.Equal(t, "sport", result.Slug)
	require.Equal(t, "rules-v2:feed+content", result.Model)
	require.Equal(t, 0.96, result.Confidence)
}

func TestFromUsesKnownPublisherCategoryWithoutContentSignal(t *testing.T) {
	result := From([]string{"දේශපාලන පුවත්"}, "නව තීරණයක් අද", "")
	require.Equal(t, "politics", result.Slug)
	require.Equal(t, "rules-v2:feed", result.Model)
}

func TestFromUsesDescriptionAndKeepsTiesDeterministic(t *testing.T) {
	result := From(nil, "නව පුවතක්", "The parliament discussed a bank")
	require.Equal(t, "politics", result.Slug)
	require.Equal(t, "rules-v2:content", result.Model)
}

func TestFromFallsBackForGeneralNewsFeed(t *testing.T) {
	result := From([]string{"News"}, "අද නව පුවතක්", "")
	require.Equal(t, "latest", result.Slug)
	require.Equal(t, 0.30, result.Confidence)
}

func TestValidSlugRejectsProviderProse(t *testing.T) {
	require.True(t, ValidSlug("economy"))
	require.False(t, ValidSlug("This looks like economy"))
}
