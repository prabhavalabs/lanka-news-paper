package politics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnalyzeSeparatesPartyPositionFromArticleFraming(t *testing.T) {
	parties := []Party{
		{Slug: "jvp", Position: -0.9, Confidence: 0.9, Aliases: []string{"JVP"}},
		{Slug: "unp", Position: 0.55, Confidence: 0.8, Aliases: []string{"UNP"}},
	}

	supportive := Analyze(parties, "JVP praised for successful reform", "")
	require.Len(t, supportive.Mentions, 1)
	require.Equal(t, "jvp", supportive.Mentions[0].PartySlug)
	require.Less(t, supportive.EconomicFrame, 0.0)
	require.Greater(t, supportive.Confidence, 0.45)

	critical := Analyze(parties, "JVP accused over failed reform", "")
	require.Greater(t, critical.EconomicFrame, 0.0)

	neutral := Analyze(parties, "UNP holds meeting in Colombo", "")
	require.Equal(t, 0.0, neutral.EconomicFrame)
	require.Less(t, neutral.Confidence, 0.45)
}

func TestAnalyzeUsesLocalContextForCompetingParties(t *testing.T) {
	parties := []Party{
		{Slug: "jvp", Position: -0.9, Confidence: 0.9, Aliases: []string{"JVP"}},
		{Slug: "unp", Position: 0.55, Confidence: 0.8, Aliases: []string{"UNP"}},
	}
	result := Analyze(parties, "JVP praised for reform while UNP accused over failure", "")
	require.Len(t, result.Mentions, 2)
	require.Less(t, result.EconomicFrame, 0.0)
}
