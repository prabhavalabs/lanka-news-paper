package storyline

import (
	"math"
	"time"
)

const (
	// Recent reports matter most; source/event caps prevent syndication dominating the score.
	recentWeight = 4
	dailyWeight  = 1
	sourceWeight = 5
	eventWeight  = 3
	sourceCap    = 12
	eventCap     = 10
	// 100 requires sustained coverage; six reports and three outlets are separate breadth gates.
	TrendThreshold     = 100
	TrendExitThreshold = 80
	MinimumArticles    = 6
	MinimumSources     = 3
)

type Metrics struct {
	Articles3h    int     `json:"articles_3h"`
	Articles6h    int     `json:"articles_6h"`
	Articles24h   int     `json:"articles_24h"`
	Sources24h    int     `json:"sources_24h"`
	Events24h     int     `json:"events_24h"`
	BaselineDaily float64 `json:"baseline_daily"`
	Acceleration  float64 `json:"acceleration"`
	Decay         float64 `json:"decay"`
}

func Score(m Metrics, last, now time.Time) (int, Metrics) {
	// One baseline day's coverage is neutral; growth reaches 1.5 at twice the baseline.
	// A floor of one avoids division by zero for a newly emerging story.
	m.Acceleration = math.Max(0.7, math.Min(1.5, 0.5+0.5*float64(m.Articles24h)/math.Max(1, m.BaselineDaily)))
	m.Decay = 1
	if age := now.Sub(last).Hours(); age > 6 {
		// Half-life of six hours after the first quiet six hours.
		m.Decay = math.Pow(0.5, (age-6)/6)
	}
	base := recentWeight*m.Articles6h + dailyWeight*m.Articles24h + sourceWeight*min(m.Sources24h, sourceCap) + eventWeight*min(m.Events24h, eventCap)
	return int(math.Round(float64(base) * m.Acceleration * m.Decay)), m
}

func IsTrending(score int, m Metrics, pinned, hidden, wasTrending bool) bool {
	return !hidden && (pinned || (wasTrending && score >= TrendExitThreshold) ||
		(score >= TrendThreshold && m.Articles24h >= MinimumArticles && m.Sources24h >= MinimumSources))
}
