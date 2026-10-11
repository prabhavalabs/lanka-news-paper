// Package frontpage assembles the public front page from the chief editor's placements, fills
// open slots automatically from recent coverage, and stores the editorials.
package frontpage

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/publish"
)

type Category struct {
	Slug   string `json:"slug"`
	NameSI string `json:"name_si"`
}

// StorySource is one outlet's report inside a story, with the link to the original article.
type StorySource struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Icon        string    `json:"icon"`
	ArticleID   string    `json:"article_id"`
	Headline    string    `json:"headline"`
	OriginalURL string    `json:"original_url"`
	PublishedAt time.Time `json:"published_at"`
}

type Story struct {
	Kind        string        `json:"kind"`
	ID          string        `json:"id"`
	Headline    string        `json:"headline"`
	Summary     *string       `json:"summary"`
	Category    *Category     `json:"category"`
	Breaking    bool          `json:"breaking"`
	PublishedAt time.Time     `json:"published_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	SourceCount int           `json:"source_count"`
	Sources     []StorySource `json:"sources"`
	Curated     bool          `json:"curated"`
	Slot        string        `json:"slot"`
	EditorNote  *string       `json:"editor_note"`
	Media       *string       `json:"media"`
}

const (
	KindEvent   = "event"
	KindArticle = "article"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// publicArticles mirrors the public news rules: published, active source, usable rights and an
// unheld category. $1 restricts to front-page sources when true.
const publicArticles = `
	SELECT a.id, a.event_id, a.headline, a.canonical_url, a.published_at, a.category_id,
	       NULLIF(btrim(document.summary_text), '') AS summary,
	       s.id AS source_id, s.name AS source_name, s.source_type, COALESCE(s.icon_url, '') AS source_icon
	FROM articles a
	JOIN sources s ON s.id = a.source_id
	JOIN rights_profiles r ON r.id = a.rights_profile_id
	LEFT JOIN categories c ON c.id = a.category_id
	LEFT JOIN article_analysis_documents document ON document.article_id = a.id
	WHERE a.public_status = 'published' AND s.active AND (NOT $1::boolean OR s.front_page)
	  AND r.mode NOT IN ('disabled', 'internal_verification')
	  AND (r.expires_at IS NULL OR r.expires_at > clock_timestamp())
	  AND (c.id IS NULL OR NOT c.held)`

// eventStories loads events by ID. Events without a qualifying article are omitted.
func (store *Store) eventStories(ctx context.Context, ids []string, frontPageOnly bool) (map[string]Story, error) {
	stories := map[string]Story{}
	if len(ids) == 0 {
		return stories, nil
	}
	rows, err := store.pool.Query(ctx, `
		WITH q AS (`+publicArticles+` AND a.event_id = ANY($2::uuid[]))
		SELECT e.id::text, e.display_title, e.is_breaking, category.slug, category.name_si,
		       min(q.published_at), max(q.published_at), count(DISTINCT q.source_id)::int,
		       COALESCE(NULLIF(btrim(analysis.summary), ''),
		                (array_agg(q.summary ORDER BY q.published_at DESC) FILTER (WHERE q.summary IS NOT NULL))[1])
		FROM event_clusters e
		JOIN q ON q.event_id = e.id
		LEFT JOIN categories category ON category.id = e.category_id
		LEFT JOIN event_narrative_analyses analysis ON analysis.event_id = e.id
		WHERE category.id IS NULL OR NOT category.held
		GROUP BY e.id, category.slug, category.name_si, analysis.summary`, frontPageOnly, ids)
	if err != nil {
		return nil, fmt.Errorf("load event stories: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		story := Story{Kind: KindEvent}
		var slug, name *string
		if err := rows.Scan(&story.ID, &story.Headline, &story.Breaking, &slug, &name, &story.PublishedAt, &story.UpdatedAt, &story.SourceCount, &story.Summary); err != nil {
			return nil, err
		}
		if slug != nil && name != nil {
			story.Category = &Category{Slug: *slug, NameSI: *name}
		}
		stories[story.ID] = story
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// One report per outlet: the outlet's earliest article on the event.
	sourceRows, err := store.pool.Query(ctx, `
		WITH q AS (`+publicArticles+` AND a.event_id = ANY($2::uuid[]))
		SELECT DISTINCT ON (q.event_id, q.source_id) q.event_id::text, q.source_id::text, q.source_name, q.source_type, q.source_icon,
		       q.id::text, q.headline, q.canonical_url, q.published_at
		FROM q ORDER BY q.event_id, q.source_id, q.published_at, q.id`, frontPageOnly, ids)
	if err != nil {
		return nil, fmt.Errorf("load event sources: %w", err)
	}
	defer sourceRows.Close()
	for sourceRows.Next() {
		var eventID string
		var source StorySource
		if err := sourceRows.Scan(&eventID, &source.ID, &source.Name, &source.Type, &source.Icon, &source.ArticleID, &source.Headline, &source.OriginalURL, &source.PublishedAt); err != nil {
			return nil, err
		}
		source.Icon = publish.PublicIcon(source.Icon)
		if story, ok := stories[eventID]; ok {
			story.Sources = append(story.Sources, source)
			stories[eventID] = story
		}
	}
	if err := sourceRows.Err(); err != nil {
		return nil, err
	}
	for id, story := range stories {
		sort.SliceStable(story.Sources, func(i, j int) bool { return story.Sources[i].PublishedAt.Before(story.Sources[j].PublishedAt) })
		stories[id] = story
	}
	return stories, nil
}

// articleStories loads single articles by ID as stories.
func (store *Store) articleStories(ctx context.Context, ids []string) (map[string]Story, error) {
	stories := map[string]Story{}
	if len(ids) == 0 {
		return stories, nil
	}
	rows, err := store.pool.Query(ctx, `
		WITH q AS (`+publicArticles+` AND a.id = ANY($2::uuid[]))
		SELECT q.id::text, q.headline, q.summary, category.slug, category.name_si, q.published_at,
		       q.source_id::text, q.source_name, q.source_type, q.source_icon, q.canonical_url
		FROM q LEFT JOIN categories category ON category.id = q.category_id`, false, ids)
	if err != nil {
		return nil, fmt.Errorf("load article stories: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		story := Story{Kind: KindArticle, SourceCount: 1}
		var slug, name *string
		var source StorySource
		if err := rows.Scan(&story.ID, &story.Headline, &story.Summary, &slug, &name, &story.PublishedAt, &source.ID, &source.Name, &source.Type, &source.Icon, &source.OriginalURL); err != nil {
			return nil, err
		}
		source.Icon = publish.PublicIcon(source.Icon)
		if slug != nil && name != nil {
			story.Category = &Category{Slug: *slug, NameSI: *name}
		}
		story.UpdatedAt = story.PublishedAt
		source.ArticleID, source.Headline, source.PublishedAt = story.ID, story.Headline, story.PublishedAt
		story.Sources = []StorySource{source}
		stories[story.ID] = story
	}
	return stories, rows.Err()
}

// candidateWindow bounds the automatic front page to recent coverage.
const candidateWindow = 36 * time.Hour

// candidates returns recent events that may fill open slots, best first.
func (store *Store) candidates(ctx context.Context, now time.Time, limit int) ([]Story, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT id::text FROM event_clusters
		WHERE last_update_at >= $1 ORDER BY last_update_at DESC LIMIT 300`, now.Add(-candidateWindow))
	if err != nil {
		return nil, fmt.Errorf("select front page candidates: %w", err)
	}
	ids := make([]string, 0, 300)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	stories, err := store.eventStories(ctx, ids, true)
	if err != nil {
		return nil, err
	}
	result := make([]Story, 0, len(stories))
	for _, story := range stories {
		result = append(result, story)
	}
	rank(result, now)
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

// sectionWeights favour the national affairs Sri Lankan readers follow most.
var sectionWeights = map[string]int{
	"politics": 14, "economy": 10, "official": 8, "local": 6, "health": 5, "crime": 4,
	"education": 4, "environment": 3, "world": 3, "technology": 2, "sport": 1, "entertainment": -4,
}

// score ranks a story for automatic placement: wider coverage, important sections and freshness.
func score(story Story, now time.Time) int {
	value := 10 * min(story.SourceCount, 6)
	if story.Category != nil {
		value += sectionWeights[story.Category.Slug]
	}
	if story.Breaking {
		value += 25
	}
	if age := now.Sub(story.UpdatedAt); age > 0 {
		value -= 3 * int(age.Hours())
	}
	return value
}

func rank(stories []Story, now time.Time) {
	sort.SliceStable(stories, func(i, j int) bool {
		a, b := score(stories[i], now), score(stories[j], now)
		if a != b {
			return a > b
		}
		if !stories[i].UpdatedAt.Equal(stories[j].UpdatedAt) {
			return stories[i].UpdatedAt.After(stories[j].UpdatedAt)
		}
		return stories[i].ID < stories[j].ID
	})
}

// Related returns up to six recent stories to read next: same section first, never the story
// itself or the same news reported in another cluster.
func (service *Service) Related(ctx context.Context, kind, id string) ([]Story, error) {
	var current Story
	var ok bool
	switch kind {
	case KindEvent:
		stories, err := service.store.eventStories(ctx, []string{id}, false)
		if err != nil {
			return nil, err
		}
		current, ok = stories[id]
	case KindArticle:
		stories, err := service.store.articleStories(ctx, []string{id})
		if err != nil {
			return nil, err
		}
		current, ok = stories[id]
	}
	if !ok {
		return nil, ErrNotFound
	}
	candidates, err := service.store.candidates(ctx, service.now(), 80)
	if err != nil {
		return nil, err
	}
	chosen := []storyWords{wordsOf(current)}
	sameSection, others := make([]Story, 0, relatedStories), make([]Story, 0, relatedStories)
	for _, story := range candidates {
		if story.Kind == current.Kind && story.ID == current.ID {
			continue
		}
		words := wordsOf(story)
		if repeats(words, chosen) {
			continue
		}
		chosen = append(chosen, words)
		if current.Category != nil && story.Category != nil && story.Category.Slug == current.Category.Slug {
			sameSection = append(sameSection, story)
		} else {
			others = append(others, story)
		}
	}
	result := append(sameSection, others...)
	if len(result) > relatedStories {
		result = result[:relatedStories]
	}
	return result, nil
}

const relatedStories = 6
