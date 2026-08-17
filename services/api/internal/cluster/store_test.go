package cluster

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSimilarityClustersSameEventAcrossSources(t *testing.T) {
	score, overlap, matches := similarity(
		"22 වැනි ව්‍යවස්ථා සංශෝධනය පාර්ලිමේන්තුවට",
		"22 වන ව්‍යවස්ථා සංශෝධන පනත් කෙටුම්පත පාර්ලිමේන්තුවේ න්‍යාය පුස්තකයට",
		0.52,
		true,
	)
	require.True(t, matches)
	require.Greater(t, overlap, 0.2)
	require.Greater(t, score, 0.48)
}

func TestSimilarityRejectsDifferentEventsAndCategory(t *testing.T) {
	_, _, matches := similarity(
		"ශ්‍රී ලංකා ක්‍රිකට් කණ්ඩායම තරගය ජය ගනී",
		"ශ්‍රී ලංකාවේ බැංකු පොලී අනුපාතය පහළට",
		0.42,
		false,
	)
	require.False(t, matches)
}

func TestTokenOverlapIgnoresCommonWords(t *testing.T) {
	require.Equal(t, 1.0, tokenOverlap("අද නව අයවැය", "අයවැය ගැන"))
}
