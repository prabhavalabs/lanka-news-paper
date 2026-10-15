package frontpage

import (
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/storyline"
	"testing"
	"time"
)

func TestRepeats(t *testing.T) {
	chosen := []storyWords{wordsOf(Story{Headline: "බන්ධනාගාර රෙගුලාසි සහ නිත්‍ය නියෝග යාවත්කාලීන කිරීමට කමිටුවක්"})}
	if !repeats(wordsOf(Story{Headline: "බන්ධනාගාර රෙගුලාසි වෙනස් කරන්න පත්කළ කමිටුවේ වාර්තාව"}), chosen) {
		t.Fatal("same story from another cluster was not detected")
	}
	if repeats(wordsOf(Story{Headline: "නේපාලයේ ගංවතුරින් ශ්‍රී ලාංකිකයින්ට බලපෑමක් නැහැ"}), chosen) {
		t.Fatal("different story treated as a repeat")
	}
}

func TestRankPrefersBreakingCoverageAndFreshness(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	politics := &Category{Slug: "politics"}
	stories := []Story{
		{ID: "stale", SourceCount: 6, Category: politics, UpdatedAt: now.Add(-30 * time.Hour)},
		{ID: "single", SourceCount: 1, UpdatedAt: now},
		{ID: "wide", SourceCount: 5, Category: politics, UpdatedAt: now.Add(-time.Hour)},
		{ID: "breaking", SourceCount: 2, Breaking: true, UpdatedAt: now},
	}
	rank(stories, now)
	got := []string{stories[0].ID, stories[1].ID, stories[2].ID, stories[3].ID}
	want := []string{"wide", "breaking", "single", "stale"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestValidatePlacements(t *testing.T) {
	now := time.Now()
	id := "6f1d8c1e-0c58-4f0e-9a51-0b8f2b0f6a10"
	past := now.Add(-time.Hour)
	cases := map[string][]PlacementInput{
		"two leads":   {{Slot: SlotLead, Kind: KindEvent, ID: id}, {Slot: SlotLead, Kind: KindArticle, ID: id}},
		"duplicate":   {{Slot: SlotTop, Kind: KindEvent, ID: id}, {Slot: SlotMore, Kind: KindEvent, ID: id}},
		"bad slot":    {{Slot: "side", Kind: KindEvent, ID: id}},
		"bad id":      {{Slot: SlotTop, Kind: KindEvent, ID: "x"}},
		"past expiry": {{Slot: SlotTop, Kind: KindEvent, ID: id, ExpiresAt: &past}},
	}
	for name, inputs := range cases {
		if err := validatePlacements(inputs, now); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if err := validatePlacements([]PlacementInput{{Slot: SlotLead, Kind: KindEvent, ID: id}, {Slot: SlotTop, Kind: KindArticle, ID: id}}, now); err != nil {
		t.Fatalf("valid placements rejected: %v", err)
	}
}

func TestTrendingStorylineBoostAndEditionFingerprint(t *testing.T) {
	now := time.Now()
	story := Story{ID: "event", UpdatedAt: now, SourceCount: 2}
	before := score(story, now)
	fingerprint := editionFingerprint([]Story{story}, nil)
	story.Storyline = &storyline.Reference{ID: "running", Title: "පුවත්", Trending: true}
	if got := score(story, now); got != before+TrendingStorylineBoost {
		t.Fatalf("score=%d, want %d", got, before+TrendingStorylineBoost)
	}
	if editionFingerprint([]Story{story}, nil) == fingerprint {
		t.Fatal("newly trending storyline must invalidate the edition")
	}
	stable := editionFingerprint([]Story{story}, nil)
	story.Storyline.Title = "නව සාරාංශය සහ නම"
	if editionFingerprint([]Story{story}, nil) != stable {
		t.Fatal("editorial wording alone must not invalidate the edition")
	}
	story.Storyline.Trending = false
	if got := score(story, now); got != before {
		t.Fatalf("inactive storyline score=%d, want %d", got, before)
	}
}
