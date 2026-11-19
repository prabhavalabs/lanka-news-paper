package trends

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/storyline"
)

var colomboTime = func() *time.Location {
	location, err := time.LoadLocation("Asia/Colombo")
	if err != nil {
		return time.FixedZone("Asia/Colombo", 5*3600+1800)
	}
	return location
}()

type Config struct {
	Store      *Store
	Storylines *storyline.Store
	Model      completer
	YouTube    YouTube
	HTTP       *http.Client
	Logger     *slog.Logger
}

// Radar ranks what Sri Lanka is paying attention to from four kinds of signal: unusually fast
// YouTube videos on Sri Lankan news channels, Google searches, live web research and the
// newsroom's own storylines (coverage speed against their 30-day baseline).
type Radar struct{ config Config }

func NewRadar(config Config) *Radar {
	if config.HTTP == nil {
		config.HTTP = &http.Client{Timeout: 30 * time.Second}
	}
	if config.YouTube.HTTP == nil {
		config.YouTube.HTTP = config.HTTP
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	return &Radar{config: config}
}

func (radar *Radar) Store() *Store { return radar.config.Store }

// StorySignal is a newsroom storyline offered to the radar.
type StorySignal struct {
	Key          string    `json:"key"`
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Latest       string    `json:"latest_development"`
	Score        int       `json:"trend_score"`
	Sources      int       `json:"outlets"`
	Articles24h  int       `json:"articles_24h"`
	Acceleration float64   `json:"acceleration_vs_30_day_baseline"`
	Trending     bool      `json:"trending"`
	LatestAt     time.Time `json:"latest_development_at"`
}

// staleAfter is how old our latest coverage of a topic may be for it to become a reel: older
// coverage would tell an outdated version of a story people are discussing today.
const staleAfter = 18 * time.Hour

// Fresh returns the latest ready run when it is younger than maxAge, otherwise runs a new pass.
func (radar *Radar) Fresh(ctx context.Context, maxAge time.Duration) (Run, error) {
	run, err := radar.config.Store.latest(ctx, true)
	if err == nil && time.Since(run.CreatedAt) < maxAge {
		return run, nil
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		return run, err
	}
	return radar.Run(ctx)
}

// Run collects every signal, lets the model group them into topics, scores and stores them.
func (radar *Radar) Run(ctx context.Context) (Run, error) {
	store := radar.config.Store
	if err := store.expireStale(ctx); err != nil {
		return Run{}, err
	}
	id, err := store.startRun(ctx)
	if err != nil {
		return Run{}, err
	}
	topics, research, signals, err := radar.rank(ctx)
	if err != nil {
		_ = store.failRun(context.WithoutCancel(ctx), id, err)
		return Run{}, err
	}
	if err := store.finishRun(ctx, id, research, signals, topics); err != nil {
		return Run{}, err
	}
	return store.latest(ctx, true)
}

func (radar *Radar) rank(ctx context.Context) ([]Topic, []WebTopic, map[string]int, error) {
	logger := radar.config.Logger
	if err := radar.Collect(ctx); err != nil {
		logger.WarnContext(ctx, "trend video refresh failed; using stored statistics", "error", err)
	}
	stories, err := radar.stories(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	google, err := FetchGoogleTrends(ctx, radar.config.HTTP)
	if err != nil {
		logger.WarnContext(ctx, "Google Trends unavailable for the radar", "error", err)
	}
	videos, err := radar.config.Store.videoSignals(ctx, 48*time.Hour, 25)
	if err != nil {
		return nil, nil, nil, err
	}
	research, err := Research(ctx, radar.config.Model, time.Now())
	if err != nil {
		logger.WarnContext(ctx, "trend web research failed", "error", err)
	}
	for i := range stories {
		stories[i].Key = fmt.Sprintf("s%d", i+1)
	}
	for i := range google {
		google[i].Key = fmt.Sprintf("g%d", i+1)
	}
	for i := range videos {
		videos[i].Key = fmt.Sprintf("y%d", i+1)
	}
	for i := range research {
		research[i].Key = fmt.Sprintf("w%d", i+1)
	}
	signals := map[string]int{"storylines": len(stories), "google_searches": len(google), "youtube_videos": len(videos), "web_topics": len(research)}
	if len(stories)+len(google)+len(videos)+len(research) == 0 {
		return nil, nil, signals, errors.New("no trend signals are available")
	}
	groups, err := mergeSignals(ctx, radar.config.Model, stories, google, videos, research)
	if err != nil {
		return nil, nil, signals, err
	}
	return scoreTopics(groups, stories, google, videos, research, time.Now()), research, signals, nil
}

// stories offers the storylines active in the last two days, strongest first.
func (radar *Radar) stories(ctx context.Context) ([]StorySignal, error) {
	all, err := radar.config.Storylines.List(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("list storylines: %w", err)
	}
	cutoff := time.Now().Add(-48 * time.Hour)
	var stories []StorySignal
	for _, s := range all {
		if s.Status != "active" || s.Hidden || s.LastActivityAt.Before(cutoff) || s.LatestEvent == nil {
			continue
		}
		stories = append(stories, StorySignal{ID: s.ID, Title: s.Title, Latest: s.LatestEvent.Title, Score: s.Score,
			Sources: s.SourceCount, Articles24h: s.Metrics.Articles24h, Acceleration: math.Round(s.Metrics.Acceleration*10) / 10, Trending: s.Trending,
			LatestAt: s.LatestEvent.Time})
	}
	sort.SliceStable(stories, func(i, j int) bool { return stories[i].Score > stories[j].Score })
	return stories[:min(len(stories), 20)], nil
}

// bulletin matches scheduled news bulletins and live streams, which cover many stories at once.
var bulletin = regexp.MustCompile(`(?i)(🔴|\blive\b|bulletin|headlines|\b\d{1,2}[.:]\d{2}\s*(am|pm)\b|\b\d{1,2}\s*-\s*\d{1,2}\s*-\s*20\d\d\b)`)

// strongestVideos keeps charted videos and those clearly ahead of their channel, best first,
// leaving out bulletins and live streams.
func strongestVideos(signals []VideoSignal, limit int) []VideoSignal {
	kept := signals[:0]
	for _, s := range signals {
		if bulletin.MatchString(s.Title) {
			continue
		}
		if s.ChartRank != nil || s.Outlier >= 1.3 {
			kept = append(kept, s)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool { return videoStrength(kept[i]) > videoStrength(kept[j]) })
	return kept[:min(len(kept), limit)]
}

func videoStrength(s VideoSignal) float64 {
	strength := math.Log2(1+s.Outlier)*2 + math.Log10(1+float64(s.Views))
	if s.ChartRank != nil {
		strength += 3 - float64(*s.ChartRank)/25
	}
	return strength
}

const mergePrompt = `You are the trend editor of Thulawa News, a balanced Sinhala news outlet in Sri Lanka. You get four kinds of signal about what Sri Lanka is paying attention to right now, each item with a key:
- storylines (s1, s2, ...): ongoing stories our newsroom covers from many outlets, with coverage speed against their 30-day baseline.
- google (g1, ...): searches rising on Google in Sri Lanka, with the headlines Google links to them.
- youtube (y1, ...): videos on Sri Lankan news channels running far ahead of their channel's normal views ("outlier" = times the usual), or on YouTube's Sri Lanka chart.
- web (w1, ...): topics a web researcher found people discussing in the last 24 hours, with momentum.

Group the signals into distinct real-world topics (one topic per story; merge signals about the same story even across Sinhala and English). For each topic:
- title_si: a short neutral Sinhala headline (at most 60 characters); title_en: the same in English.
- category: politics, economy, courts, crime, sports, entertainment, society, disaster, international or other.
- storyline: the key of our storyline that covers this topic, or "" if none does. Only match when it is truly the same story.
- youtube, google, web: the keys of the signals that belong to this topic (may be empty). Do not reuse a key in two topics.
- suitable: whether Thulawa may turn it into a short Facebook Reel that ends with a Like/Love yes-or-no debate poll. The poll is always about opinion, satisfaction, trust or public policy, never about anyone's guilt, so court cases involving public figures, political controversies, viral incidents with a policy angle (for example tougher penalties or better regulation), economy, sport and entertainment are suitable. Not suitable: stories centred on deaths, injuries, disasters or accidents; violent crime against private individuals; children; sexual content; private people's personal lives; or anything where a reaction poll would be insensitive.
- reason: one sentence on why people are paying attention, or why it is not suitable.
Leave out signals that are not news about Sri Lanka or of clear interest to Sri Lankans (for example foreign entertainment or music videos), and topics supported by nothing. At most 15 topics. Never invent facts beyond the signals.`

type group struct {
	TitleSI   string   `json:"title_si"`
	TitleEN   string   `json:"title_en"`
	Category  string   `json:"category"`
	Storyline string   `json:"storyline"`
	YouTube   []string `json:"youtube"`
	Google    []string `json:"google"`
	Web       []string `json:"web"`
	Suitable  bool     `json:"suitable"`
	Reason    string   `json:"reason"`
}

func mergeSchema() map[string]any {
	str := func(max int) map[string]any { return map[string]any{"type": "string", "maxLength": max} }
	keys := map[string]any{"type": "array", "maxItems": 12, "items": str(5)}
	return map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"topics"},
		"properties": map[string]any{"topics": map[string]any{"type": "array", "maxItems": 15, "items": map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []string{"title_si", "title_en", "category", "storyline", "youtube", "google", "web", "suitable", "reason"},
			"properties": map[string]any{
				"title_si": str(90), "title_en": str(140), "category": map[string]any{"type": "string", "enum": categories},
				"storyline": str(5), "youtube": keys, "google": keys, "web": keys,
				"suitable": map[string]any{"type": "boolean"}, "reason": str(300),
			},
		}}},
	}
}

func mergeSignals(ctx context.Context, model completer, stories []StorySignal, google []GoogleTrend, videos []VideoSignal, research []WebTopic) ([]group, error) {
	type briefVideo struct {
		Key       string  `json:"key"`
		Title     string  `json:"title"`
		Channel   string  `json:"channel"`
		Views     int64   `json:"views"`
		AgeHours  int     `json:"age_hours"`
		Outlier   float64 `json:"outlier"`
		ChartRank *int    `json:"chart_rank,omitempty"`
	}
	brief := make([]briefVideo, len(videos))
	for i, v := range videos {
		brief[i] = briefVideo{v.Key, v.Title, v.Channel, v.Views, int(v.AgeHours), math.Round(v.Outlier*10) / 10, v.ChartRank}
	}
	data, _ := json.Marshal(map[string]any{"storylines": stories, "google": google, "youtube": brief, "web": research})
	response, err := model.Complete(ctx, llm.Request{Task: "trend_radar", System: mergePrompt, Input: string(data), JSONSchema: mergeSchema()})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(response.Text) == "" {
		return nil, fmt.Errorf("the trend_radar AI task returned nothing; check its route under AI routing")
	}
	var result struct {
		Topics []group `json:"topics"`
	}
	if err := json.Unmarshal([]byte(response.Text), &result); err != nil {
		return nil, fmt.Errorf("decode trend radar: %w", err)
	}
	return result.Topics, nil
}

var trafficNumber = regexp.MustCompile(`[\d,.]+\s*[KkMm]?`)

// searches turns Google's "2000+" or "10K+" into a number.
func searches(traffic string) float64 {
	match := strings.TrimSpace(trafficNumber.FindString(traffic))
	if match == "" {
		return 0
	}
	multiplier := 1.0
	switch strings.ToUpper(match[len(match)-1:]) {
	case "K":
		multiplier, match = 1e3, strings.TrimSpace(match[:len(match)-1])
	case "M":
		multiplier, match = 1e6, strings.TrimSpace(match[:len(match)-1])
	}
	value, _ := strconv.ParseFloat(strings.ReplaceAll(match, ",", ""), 64)
	return value * multiplier
}

// scoreTopics resolves each group's keys to its evidence and scores it in code, so every rank can
// be explained: YouTube (how far ahead of normal and how many views), search volume, web research
// (rising topics count more), newsroom coverage, and a bonus when independent sources agree.
func scoreTopics(groups []group, stories []StorySignal, google []GoogleTrend, videos []VideoSignal, research []WebTopic, now time.Time) []Topic {
	storyByKey := map[string]StorySignal{}
	for _, s := range stories {
		storyByKey[s.Key] = s
	}
	googleByKey := map[string]GoogleTrend{}
	for _, g := range google {
		googleByKey[g.Key] = g
	}
	videoByKey := map[string]VideoSignal{}
	for _, v := range videos {
		videoByKey[v.Key] = v
	}
	webByKey := map[string]WebTopic{}
	for _, w := range research {
		webByKey[w.Key] = w
	}
	used := map[string]bool{}
	take := func(key string) bool {
		key = strings.TrimSpace(key)
		if used[key] {
			return false
		}
		used[key] = true
		return true
	}
	topics := make([]Topic, 0, len(groups))
	for _, g := range groups {
		topic := Topic{TitleSI: g.TitleSI, TitleEN: g.TitleEN, Category: g.Category, Reason: g.Reason,
			Evidence: Evidence{YouTube: []VideoSignal{}, Google: []GoogleTrend{}, Web: []WebTopic{}}}
		var bestOutlier, views float64
		charted := false
		for _, key := range g.YouTube {
			if v, ok := videoByKey[strings.TrimSpace(key)]; ok && take(key) {
				topic.Evidence.YouTube = append(topic.Evidence.YouTube, v)
				bestOutlier, views = math.Max(bestOutlier, v.Outlier), views+float64(v.Views)
				charted = charted || v.ChartRank != nil
			}
		}
		if len(topic.Evidence.YouTube) > 0 {
			// Capped below the other signals combined: one channel's spike alone should not outrank
			// a story several independent sources agree on.
			topic.Components.YouTube = math.Min(35, 12*math.Log2(1+math.Min(bestOutlier, 50))+2*math.Log10(1+views))
			if charted {
				topic.Components.YouTube += 5
			}
		}
		var searchCount float64
		for _, key := range g.Google {
			if t, ok := googleByKey[strings.TrimSpace(key)]; ok && take(key) {
				topic.Evidence.Google = append(topic.Evidence.Google, t)
				searchCount += searches(t.Traffic)
			}
		}
		if len(topic.Evidence.Google) > 0 {
			topic.Components.Search = math.Min(35, 8*math.Log10(1+searchCount)+5)
		}
		for _, key := range g.Web {
			if w, ok := webByKey[strings.TrimSpace(key)]; ok && take(key) {
				topic.Evidence.Web = append(topic.Evidence.Web, w)
				topic.Components.Web += map[string]float64{"rising": 15, "steady": 10, "fading": 5}[w.Momentum]
			}
		}
		topic.Components.Web = math.Min(30, topic.Components.Web)
		if s, ok := storyByKey[strings.TrimSpace(g.Storyline)]; ok && take(g.Storyline) {
			story := s
			topic.Evidence.Storyline = &story
			topic.StorylineID = &story.ID
			topic.Components.Coverage = math.Min(40, float64(s.Score)/5+math.Min(10, 2*math.Max(0, s.Acceleration)))
		}
		sources := 0
		for _, value := range []float64{topic.Components.YouTube, topic.Components.Search, topic.Components.Web, topic.Components.Coverage} {
			if value > 0 {
				sources++
			}
		}
		if sources == 0 {
			continue
		}
		topic.Components.Agreement = 15 * float64(sources-1)
		c := topic.Components
		topic.Score = math.Round((c.YouTube+c.Search+c.Web+c.Coverage+c.Agreement)*10) / 10
		switch {
		case !g.Suitable:
			topic.Status = "skip"
		case topic.Evidence.Storyline != nil && topic.Evidence.Storyline.Sources >= 2 && now.Sub(topic.Evidence.Storyline.LatestAt) <= staleAfter:
			topic.Status = "ready"
		case topic.Evidence.Storyline != nil && topic.Evidence.Storyline.Sources >= 2:
			topic.Status = "watch"
			topic.Reason += fmt.Sprintf(" Our latest coverage is %d hours old, so it waits for a newer report.", int(now.Sub(topic.Evidence.Storyline.LatestAt).Hours()))
		default:
			topic.Status = "watch"
		}
		topics = append(topics, topic)
	}
	sort.SliceStable(topics, func(i, j int) bool { return topics[i].Score > topics[j].Score })
	for i := range topics {
		topics[i].Rank = i + 1
	}
	return topics
}
