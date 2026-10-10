package social

import (
	"context"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	candidatePoolFactor = 3
	// Recent posts the editor and the duplicate check compare new candidates with.
	recentPostWindow = 48 * time.Hour
	// At most topicCapPosts non-breaking posts from one category within topicCapWindow publish
	// automatically; further ones wait for an editor so the Page does not repeat one kind of story.
	topicCapWindow = 6 * time.Hour
	topicCapPosts  = 2
	// A post approved automatically but not yet published is dropped once its story is this old.
	autoPublishMaxAge = 12 * time.Hour
	// Word-overlap thresholds above which a draft repeats a recent post.
	headlineOverlap = 0.5
	summaryOverlap  = 0.45
)

// topicWeights reflect what draws Sri Lankan readers: politics and governance first, then the
// economy, official notices and national affairs. Breaking news is ranked ahead of all of these.
var topicWeights = map[string]int{
	"politics": 40, "economy": 25, "official": 20, "local": 12, "health": 10, "crime": 6,
	"education": 6, "environment": 4, "world": 2, "technology": 0, "latest": 0, "sport": -5, "entertainment": -15,
}

// priority scores a candidate for selection order; higher goes first.
func priority(c Candidate, now time.Time) int {
	score := topicWeights[c.CategorySlug] + 8*min(c.SourceCount, 8) + 2*min(c.ArticleCount, 10)
	if !c.LatestAt.IsZero() && now.After(c.LatestAt) {
		score -= 4 * min(int(now.Sub(c.LatestAt).Hours()), 24)
	}
	return score
}

// recentPost is a post already on, or headed for, the Page.
type recentPost struct {
	ID           string    `json:"-"`
	Headline     string    `json:"headline"`
	Summary      string    `json:"-"`
	Category     string    `json:"category"`
	CategorySlug string    `json:"-"`
	Breaking     bool      `json:"breaking"`
	Status       string    `json:"status"`
	At           time.Time `json:"at"`
}

func (store *Store) recentPosts(ctx context.Context, now time.Time) ([]recentPost, error) {
	rows, err := store.pool.Query(ctx, `
 SELECT p.id::text,p.headline_si,p.summary_si,COALESCE(c.name_si,''),COALESCE(c.slug,''),p.kind='breaking',p.status,
 COALESCE(p.published_at,p.created_at)
 FROM social_posts p
 LEFT JOIN event_clusters e ON e.id=p.event_id LEFT JOIN articles a ON a.id=p.article_id
 LEFT JOIN categories c ON c.id=COALESCE(e.category_id,a.category_id)
 WHERE p.status IN ('pending_review','approved','publishing','published') AND p.created_at>=$1
 ORDER BY COALESCE(p.published_at,p.created_at) DESC LIMIT 60`, now.Add(-recentPostWindow))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]recentPost, 0)
	for rows.Next() {
		var item recentPost
		if err := rows.Scan(&item.ID, &item.Headline, &item.Summary, &item.Category, &item.CategorySlug, &item.Breaking, &item.Status, &item.At); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// words returns the distinct meaningful words of Sinhala or English text.
func words(text string) map[string]bool {
	result := map[string]bool{}
	for _, word := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r) && r != '‍'
	}) {
		if utf8.RuneCountInString(word) >= 3 {
			result[word] = true
		}
	}
	return result
}

// overlap is the share of the smaller text's words that also appear in the other.
func overlap(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	shared := 0
	for word := range a {
		if b[word] {
			shared++
		}
	}
	return float64(shared) / float64(min(len(a), len(b)))
}

// duplicateOf returns the recent post a draft repeats, if any.
func duplicateOf(item editorialItem, recent []recentPost) *recentPost {
	headline, summary := words(item.HeadlineSI), words(item.SummarySI)
	for i := range recent {
		if overlap(headline, words(recent[i].Headline)) >= headlineOverlap || overlap(summary, words(recent[i].Summary)) >= summaryOverlap {
			return &recent[i]
		}
	}
	return nil
}

// topicHold explains why a non-breaking post should wait for an editor because its category
// already had topicCapPosts posts within topicCapWindow. It returns "" when it may publish.
func topicHold(candidate Candidate, recent []recentPost, now time.Time) string {
	if candidate.Breaking || candidate.CategorySlug == "" {
		return ""
	}
	count := 0
	for _, post := range recent {
		if post.Status != "pending_review" && post.CategorySlug == candidate.CategorySlug && post.At.After(now.Add(-topicCapWindow)) {
			count++
		}
	}
	if count < topicCapPosts {
		return ""
	}
	return "Held for an editor: " + candidate.Category + " already had " + strconv.Itoa(count) + " posts in the last 6 hours. Approve it if it is important enough to post now."
}
