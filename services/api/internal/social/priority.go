package social

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	candidatePoolFactor = 3
	// Recent posts the editor and the duplicate check compare new candidates with.
	recentPostWindow = 48 * time.Hour
	// Each post from the same category published within topicWindow lowers a queued post's rank
	// by topicPenalty, so the Page does not run one kind of story back to back.
	topicWindow  = 6 * time.Hour
	topicPenalty = 15
	// A post approved automatically but not yet published is dropped once its story is this old.
	autoPublishMaxAge = 12 * time.Hour
	// Breaking news and posts scored at least priorityScore go out even after the daily limit;
	// breaking news also ignores quiet hours. Nothing important waits for tomorrow.
	priorityScore = 85
	// An automatic post failing review is rewritten up to maxAutoRevisions times before it is dropped.
	maxAutoRevisions = 2
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

// pageActivity tells the editor how busy the Page has been, so a quiet Page gets a steady flow.
// It carries no limits: the publisher enforces them, so the editor judges news value alone.
type pageActivity struct {
	PostsLast24Hours   int  `json:"posts_last_24_hours"`
	HoursSinceLastPost *int `json:"hours_since_last_post"`
}

func activityOf(recent []recentPost, now time.Time) pageActivity {
	var result pageActivity
	var last time.Time
	for _, post := range recent {
		if post.Status == "pending_review" {
			continue
		}
		if post.At.After(now.Add(-24 * time.Hour)) {
			result.PostsLast24Hours++
		}
		if post.At.After(last) {
			last = post.At
		}
	}
	if !last.IsZero() {
		hours := int(now.Sub(last).Hours())
		result.HoursSinceLastPost = &hours
	}
	return result
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

// queuedPost is an approved post waiting for a publishing slot.
type queuedPost struct {
	ID           string
	Breaking     bool
	Score        int
	CategorySlug string
	Headline     string
	Summary      string
	CreatedAt    time.Time
}

// rank orders the publishing queue: breaking news first, then the most important story,
// lowered for categories the Page has just carried and for stories that are getting old.
func (post queuedPost) rank(published []recentPost, now time.Time) int {
	score := post.Score
	if post.Breaking {
		score += 100
	}
	for _, recent := range published {
		if recent.CategorySlug != "" && recent.CategorySlug == post.CategorySlug && recent.At.After(now.Add(-topicWindow)) {
			score -= topicPenalty
		}
	}
	return score - 2*int(now.Sub(post.CreatedAt).Hours())
}

// limitsFor relaxes the Page limits for important posts: breaking news and high scores may
// pass the daily limit, and breaking news may also publish during quiet hours.
func limitsFor(post queuedPost, settings Settings) Settings {
	if post.Breaking || post.Score >= priorityScore {
		settings.MaxPostsPerDay = 1000
	}
	if post.Breaking {
		settings.QuietHoursEnd = settings.QuietHoursStart
	}
	return settings
}

// repeatsPublished returns the published post a queued post repeats, if any.
func repeatsPublished(post queuedPost, published []recentPost) *recentPost {
	return duplicateOf(editorialItem{HeadlineSI: post.Headline, SummarySI: post.Summary}, published)
}
