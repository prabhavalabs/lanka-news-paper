package storyline

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestScoreAndTrending(t *testing.T) {
	now := time.Now()
	m := Metrics{Articles3h: 6, Articles6h: 12, Articles24h: 18, Sources24h: 6, Events24h: 4, BaselineDaily: 9}
	score, detail := Score(m, now, now)
	require.Equal(t, 162, score)
	require.Equal(t, 1.5, detail.Acceleration)
	require.True(t, IsTrending(score, detail, false, false, false))
	require.False(t, IsTrending(99, detail, false, false, false))
	require.True(t, IsTrending(100, detail, false, false, false))
	few := detail
	few.Articles24h = 5
	require.False(t, IsTrending(100, few, false, false, false))
	few = detail
	few.Sources24h = 2
	require.False(t, IsTrending(100, few, false, false, false))
	require.True(t, IsTrending(0, Metrics{}, true, false, false))
	require.False(t, IsTrending(1000, detail, true, true, false))
	decayed, _ := Score(m, now.Add(-12*time.Hour), now)
	require.Equal(t, 81, decayed)
	m.BaselineDaily = 1000
	_, detail = Score(m, now, now)
	require.Equal(t, 0.7, detail.Acceleration)
	m.BaselineDaily = 0
	_, detail = Score(m, now, now)
	require.Equal(t, 1.5, detail.Acceleration)
	m.Sources24h = 120
	m.Events24h = 100
	capped, _ := Score(m, now, now)
	m.Sources24h = 12
	m.Events24h = 10
	limit, _ := Score(m, now, now)
	require.Equal(t, limit, capped)
}

func TestBurstTermsDistinctArticlesAndSinhala(t *testing.T) {
	now := time.Now()
	headlines := []Headline{
		{"a", "ගමන ගමන සඳහා Protest", now}, {"a", "ගමන Protest", now},
		{"b", "ගමන සඳහා PROTEST", now}, {"c", "ගමන සඳහා Protest", now},
		{"old", "ගමන", now.Add(-2 * 24 * time.Hour)},
		{"outside", "ignored", now.Add(-9 * 24 * time.Hour)},
		{"future", "ignored", now.Add(time.Hour)},
	}
	terms := BurstTerms(headlines, now)
	require.Len(t, terms, 2)
	for _, term := range terms {
		require.Equal(t, 3, term.Articles24h)
		if term.Term == "ගමන" {
			require.InDelta(t, 1.0/7, term.BaselineDaily, 0.001)
		}
	}
}

func TestTrendingHysteresis(t *testing.T) {
	m := Metrics{Articles24h: MinimumArticles, Sources24h: MinimumSources}
	require.False(t, IsTrending(99, m, false, false, false))
	require.True(t, IsTrending(100, m, false, false, false))
	require.True(t, IsTrending(99, m, false, false, true))
	require.True(t, IsTrending(80, Metrics{}, false, false, true))
	require.False(t, IsTrending(79, m, false, false, true))
	require.False(t, IsTrending(100, m, true, true, true))
}
