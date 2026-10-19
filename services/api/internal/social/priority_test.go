package social

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestDuplicateOf(t *testing.T) {
	recent := []recentPost{{Headline: "නාගොඩ වෙඩි තැබීම සම්බන්ධයෙන් සැකකරුවෙකු අත්අඩංගුවට", Summary: "අවන්හල් හිමිකරුවෙකු මියගිය වෙඩි තැබීම"}}
	repeat := editorialItem{HeadlineSI: "නාගොඩ වෙඩි තැබීම: සැකකරුවෙකු අත්අඩංගුවට", SummarySI: "පොලිසිය පවසයි"}
	require.NotNil(t, duplicateOf(repeat, recent))
	other := editorialItem{HeadlineSI: "පොල්ගහවෙල–පේරාදෙණිය දිනපතා දුම්රියක්", SummarySI: "පාසල් සිසුන් වෙනුවෙන් නව දුම්රිය සේවාවක්"}
	require.Nil(t, duplicateOf(other, recent))
	require.Nil(t, duplicateOf(repeat, nil))
}

func TestQueueRank(t *testing.T) {
	now := time.Now()
	published := []recentPost{
		{CategorySlug: "crime", Status: "published", At: now.Add(-time.Hour)},
		{CategorySlug: "crime", Status: "published", At: now.Add(-2 * time.Hour)},
		{CategorySlug: "politics", Status: "published", At: now.Add(-8 * time.Hour)},
	}
	crime := queuedPost{Score: 90, CategorySlug: "crime", CreatedAt: now}
	politics := queuedPost{Score: 80, CategorySlug: "politics", CreatedAt: now}
	require.Greater(t, politics.rank(published, now), crime.rank(published, now))
	breaking := queuedPost{Score: 60, Breaking: true, CategorySlug: "crime", CreatedAt: now}
	require.Greater(t, breaking.rank(published, now), politics.rank(published, now))
	old := queuedPost{Score: 80, CategorySlug: "politics", CreatedAt: now.Add(-6 * time.Hour)}
	require.Greater(t, politics.rank(published, now), old.rank(published, now))
}

func TestLimitsFor(t *testing.T) {
	settings := Settings{MaxPostsPerDay: 12, MaxPostsPerHour: 2, MinGapMinutes: 20, QuietHoursStart: 0, QuietHoursEnd: 6}
	require.Equal(t, settings, limitsFor(queuedPost{Score: 70}, settings))
	important := limitsFor(queuedPost{Score: priorityScore}, settings)
	require.Greater(t, important.MaxPostsPerDay, 12)
	require.Greater(t, important.MaxPostsPerHour, 2)
	require.Equal(t, 6, int(important.QuietHoursEnd))
	require.Equal(t, 20, important.MinGapMinutes)
	breaking := limitsFor(queuedPost{Score: 50, Breaking: true}, settings)
	require.Greater(t, breaking.MaxPostsPerDay, 12)
	require.Equal(t, breaking.QuietHoursStart, breaking.QuietHoursEnd)
	require.Equal(t, breakingGapMinutes, breaking.MinGapMinutes)
}

func TestRepeatsPublished(t *testing.T) {
	published := []recentPost{{Headline: "නාගොඩ වෙඩි තැබීම සම්බන්ධයෙන් සැකකරුවෙකු අත්අඩංගුවට", Summary: "අවන්හල් හිමිකරුවෙකු මියගිය වෙඩි තැබීම"}}
	require.NotNil(t, repeatsPublished(queuedPost{Headline: "නාගොඩ වෙඩි තැබීම: සැකකරුවෙකු අත්අඩංගුවට"}, published))
	require.Nil(t, repeatsPublished(queuedPost{Headline: "පොල්ගහවෙල–පේරාදෙණිය දිනපතා දුම්රියක්"}, published))
}

func TestCaptionForReviewDropsOwnLink(t *testing.T) {
	p := Post{LinkURL: "https://thulawanews.com/e/1", Caption: BuildCaption("පුවත.", []string{"Hiru News"}, "https://thulawanews.com/e/1", []string{"දුම්රිය"})}
	require.NotContains(t, captionForReview(p), "thulawanews.com")
	require.Contains(t, captionForReview(p), "වාර්තා කළේ: Hiru News\n\n#දුම්රිය")
}

func TestPlanFor(t *testing.T) {
	settings := Settings{Enabled: true, AutoPublish: true}
	failed := planFor(Post{Status: "pending_review", ReviewStatus: "failed"}, settings, true, nil)
	require.Equal(t, "needs_editor", failed.State)
	require.Contains(t, failed.Note, "Will not publish automatically")
	at := time.Now().Add(time.Hour)
	scheduled := planFor(Post{ID: "a", Status: "approved"}, settings, true, map[string]time.Time{"a": at})
	require.Equal(t, "scheduled", scheduled.State)
	require.True(t, scheduled.Estimated)
	require.Equal(t, at, *scheduled.At)
	paused := planFor(Post{ID: "a", Status: "approved"}, Settings{}, true, nil)
	require.Equal(t, "paused", paused.State)
}

func TestRevisionHelpers(t *testing.T) {
	require.Equal(t, "පුවත.", captionBody(BuildCaption("පුවත.", []string{"Hiru News"}, "", []string{"දුම්රිය"})))
	require.True(t, revisable(reviewResult{Issues: []ReviewIssue{{Severity: "major"}}}, 80))
	require.True(t, revisable(reviewResult{Score: 60, Issues: []ReviewIssue{{Severity: "minor"}}}, 80))
	require.True(t, revisable(reviewResult{Score: 60}, 80))
	require.False(t, revisable(reviewResult{Score: 90, Issues: []ReviewIssue{{Severity: "minor"}}}, 80))
	require.False(t, revisable(reviewResult{Repeats: true, Issues: []ReviewIssue{{Severity: "major"}}}, 80))
	require.False(t, approvable(reviewResult{Pass: true, Score: 95, Repeats: true}, 80))
	require.Equal(t, "thulawanews.com", siteName("https://www.thulawanews.com/"))
}

func TestActivityOf(t *testing.T) {
	now := time.Date(2026, 10, 11, 8, 0, 0, 0, time.UTC)
	quiet := activityOf(nil, now)
	if quiet.PostsLast24Hours != 0 || quiet.HoursSinceLastPost != nil {
		t.Fatalf("quiet page = %+v", quiet)
	}
	recent := []recentPost{
		{Status: "published", At: now.Add(-5 * time.Hour)},
		{Status: "approved", At: now.Add(-3 * time.Hour)},
		{Status: "pending_review", At: now.Add(-1 * time.Hour)},
		{Status: "published", At: now.Add(-30 * time.Hour)},
	}
	busy := activityOf(recent, now)
	if busy.PostsLast24Hours != 2 || busy.HoursSinceLastPost == nil || *busy.HoursSinceLastPost != 3 {
		t.Fatalf("activity = %+v", busy)
	}
}

func TestTrendingStorylineRepeatCooldownAndPriority(t *testing.T) {
	now := time.Now()
	story := &StorylineInfo{ID: "saga", Trending: true}
	item := editorialItem{HeadlineSI: "ගමන කොළඹ අවසන් වී ඇත", SummarySI: "ජනාධිපති ලේකම් වෙත සංදේශයක්", Storyline: story, At: now}
	recent := []recentPost{{ID: "old", Headline: item.HeadlineSI, Summary: item.SummarySI, StorylineID: "saga", Status: "published", At: now.Add(-4 * time.Hour)}}
	require.NotNil(t, duplicateOf(item, recent), "identical development still repeats after cooldown")
	recent[0].Headline = "ගමන කොළඹ අවසන් වීම"
	require.NotNil(t, duplicateOf(item, recent), "near-identical development still repeats")
	recent[0].Headline = "අධිකරණ තීන්දුව නිකුත් කළා"
	recent[0].Summary = "සැකකරු ඇප මත මුදා හැරේ"
	require.Nil(t, duplicateOf(item, recent), "genuinely different development after cooldown")
	recent[0].Summary = item.SummarySI
	require.Nil(t, duplicateOf(item, recent), "shared saga background in the summary is not a repeat")
	recent[0].Summary = "සැකකරු ඇප මත මුදා හැරේ"
	recent[0].At = now.Add(-3 * time.Hour)
	require.NotNil(t, duplicateOf(item, recent), "exact cooldown boundary is protected")
	recent[0].At = now.Add(-time.Hour)
	recent[0].Headline = "වෙනස් පුවත"
	recent[0].Summary = "වෙනස් වාර්තාව"
	require.NotNil(t, duplicateOf(item, recent), "same storyline within cooldown blocks even low word overlap")
	recent[0].At = now.Add(-4 * time.Hour)
	recent[0].Status = "approved"
	require.NotNil(t, duplicateOf(item, recent))
	recent[0].Status = "published"
	recent[0].StorylineID = "other"
	recent[0].Headline = item.HeadlineSI
	require.NotNil(t, duplicateOf(item, recent), "unrelated storyline never gets exemption")
	base := Candidate{SourceCount: 2, ArticleCount: 3, LatestAt: now}
	before := priority(base, now)
	base.Storyline = story
	require.Equal(t, before+trendingStorylineBoost, priority(base, now))
	story.Trending = false
	require.Equal(t, before, priority(base, now))
}

func TestBreakingStorylineUsesNormalOverlap(t *testing.T) {
	now := time.Now()
	story := &StorylineInfo{ID: "saga", Trending: true}
	prior := recentPost{StorylineID: "saga", Status: "published", At: now.Add(-time.Hour), Headline: "අධිකරණ තීන්දුව නිකුත් කළා"}
	item := editorialItem{Storyline: story, At: now, HeadlineSI: "ගමන කොළඹ අවසන් වී ඇත"}
	require.NotNil(t, duplicateOf(item, []recentPost{prior}))
	item.Breaking = true
	require.Nil(t, duplicateOf(item, []recentPost{prior}))
	require.Nil(t, repeatsPublished(queuedPost{Storyline: story, Now: now, Breaking: true, Headline: item.HeadlineSI}, []recentPost{prior}))
	prior.Headline = item.HeadlineSI
	require.NotNil(t, duplicateOf(item, []recentPost{prior}), "breaking still rejects identical developments")
	item.Breaking = false
	prior.Breaking = true
	prior.Headline = "අධිකරණ තීන්දුව නිකුත් කළා"
	require.Nil(t, duplicateOf(item, []recentPost{prior}), "earlier breaking posts do not impose saga spacing")
}

func TestLimitCaptionKeepsLineBreaks(t *testing.T) {
	caption := "පුවත.\n\nවාර්තා කළේ: Hiru News"
	require.Equal(t, caption, limitCaption(caption))
	long := strings.Repeat("අ", maxCaptionRunes+50)
	require.Equal(t, maxCaptionRunes, utf8.RuneCountInString(limitCaption(long)))
}
