package social

import (
	"testing"
	"time"

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
	require.Contains(t, captionForReview(p), "මූලාශ්‍ර: Hiru News\n\n#දුම්රිය")
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
