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

func TestTopicHold(t *testing.T) {
	now := time.Now()
	recent := []recentPost{
		{CategorySlug: "crime", Status: "published", At: now.Add(-time.Hour)},
		{CategorySlug: "crime", Status: "approved", At: now.Add(-2 * time.Hour)},
		{CategorySlug: "crime", Status: "pending_review", At: now},
		{CategorySlug: "politics", Status: "published", At: now.Add(-time.Hour)},
	}
	require.NotEmpty(t, topicHold(Candidate{CategorySlug: "crime", Category: "අපරාධ"}, recent, now))
	require.Empty(t, topicHold(Candidate{CategorySlug: "crime", Breaking: true}, recent, now))
	require.Empty(t, topicHold(Candidate{CategorySlug: "politics"}, recent, now))
	require.Empty(t, topicHold(Candidate{CategorySlug: "crime"}, recent, now.Add(6*time.Hour)))
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
