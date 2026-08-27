package newsletter

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type Story struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"`
	Title        string    `json:"title"`
	Summary      string    `json:"summary"`
	Category     string    `json:"category"`
	URL          string    `json:"url"`
	Breaking     bool      `json:"breaking"`
	ArticleCount int       `json:"article_count"`
	SourceCount  int       `json:"source_count"`
	LatestAt     time.Time `json:"latest_at"`
}

type Digest struct {
	EditionDate  string    `json:"edition_date"`
	WindowStart  time.Time `json:"window_start"`
	WindowEnd    time.Time `json:"window_end"`
	ArticleCount int       `json:"article_count"`
	EventCount   int       `json:"event_count"`
	SourceCount  int       `json:"source_count"`
	Stories      []Story   `json:"stories"`
}

func (store *Store) BuildDigest(ctx context.Context, editionDate string, start, end time.Time, baseURL string) (Digest, error) {
	digest := Digest{EditionDate: editionDate, WindowStart: start, WindowEnd: end, Stories: make([]Story, 0)}
	if err := store.pool.QueryRow(ctx, `
		SELECT count(*)::integer,
		       count(DISTINCT article.event_id) FILTER (WHERE article.event_id IS NOT NULL)::integer,
		       count(DISTINCT article.source_id)::integer
		FROM articles article
		JOIN sources source ON source.id = article.source_id
		JOIN rights_profiles rights ON rights.id = article.rights_profile_id
		LEFT JOIN categories category ON category.id = article.category_id
		WHERE article.public_status = 'published'
		  AND article.published_at >= $1 AND article.published_at < $2
		  AND source.active
		  AND rights.mode NOT IN ('disabled', 'internal_verification')
		  AND (rights.expires_at IS NULL OR rights.expires_at > clock_timestamp())
		  AND (category.id IS NULL OR NOT category.held)
	`, start, end).Scan(&digest.ArticleCount, &digest.EventCount, &digest.SourceCount); err != nil {
		return Digest{}, fmt.Errorf("count newsletter coverage: %w", err)
	}
	events, err := store.eventStories(ctx, start, end, baseURL)
	if err != nil {
		return Digest{}, err
	}
	articles, err := store.singleArticleStories(ctx, start, end, baseURL)
	if err != nil {
		return Digest{}, err
	}
	digest.Stories = append(digest.Stories, events...)
	digest.Stories = append(digest.Stories, articles...)
	sort.SliceStable(digest.Stories, func(left, right int) bool {
		a, b := digest.Stories[left], digest.Stories[right]
		if a.Breaking != b.Breaking {
			return a.Breaking
		}
		if a.SourceCount != b.SourceCount {
			return a.SourceCount > b.SourceCount
		}
		if a.ArticleCount != b.ArticleCount {
			return a.ArticleCount > b.ArticleCount
		}
		return a.LatestAt.After(b.LatestAt)
	})
	if len(digest.Stories) > 30 {
		digest.Stories = digest.Stories[:30]
	}
	return digest, nil
}

func (store *Store) eventStories(ctx context.Context, start, end time.Time, baseURL string) ([]Story, error) {
	rows, err := store.pool.Query(ctx, `
		WITH qualifying AS (
		  SELECT article.*
		  FROM articles article
		  JOIN sources source ON source.id = article.source_id
		  JOIN rights_profiles rights ON rights.id = article.rights_profile_id
		  LEFT JOIN categories category ON category.id = article.category_id
		  WHERE article.public_status = 'published'
		    AND article.published_at >= $1 AND article.published_at < $2
		    AND source.active
		    AND rights.mode NOT IN ('disabled', 'internal_verification')
		    AND (rights.expires_at IS NULL OR rights.expires_at > clock_timestamp())
		    AND (category.id IS NULL OR NOT category.held)
		)
		SELECT event.id::text, event.display_title,
		       COALESCE(category.name_si, 'වෙනත්'), event.is_breaking,
		       max(article.published_at), count(DISTINCT article.id)::integer,
		       count(DISTINCT article.source_id)::integer,
		       COALESCE(NULLIF(analysis.summary, ''), NULLIF(latest.summary_text, ''),
		                NULLIF(latest.description, ''), event.display_title)
		FROM qualifying article
		JOIN event_clusters event ON event.id = article.event_id
		LEFT JOIN categories category ON category.id = event.category_id
		LEFT JOIN event_narrative_analyses analysis ON analysis.event_id = event.id
		LEFT JOIN LATERAL (
		  SELECT document.summary_text, recent.description
		  FROM qualifying recent
		  LEFT JOIN article_analysis_documents document ON document.article_id = recent.id
		  WHERE recent.event_id = event.id
		  ORDER BY recent.published_at DESC, recent.id DESC
		  LIMIT 1
		) latest ON true
		GROUP BY event.id, category.name_si, analysis.summary, latest.summary_text, latest.description
		ORDER BY event.is_breaking DESC, count(DISTINCT article.source_id) DESC,
		         count(DISTINCT article.id) DESC, max(article.published_at) DESC
		LIMIT 30
	`, start, end)
	if err != nil {
		return nil, fmt.Errorf("select newsletter events: %w", err)
	}
	defer rows.Close()
	stories := make([]Story, 0)
	for rows.Next() {
		var story Story
		story.Kind = "event"
		if err := rows.Scan(
			&story.ID, &story.Title, &story.Category, &story.Breaking, &story.LatestAt,
			&story.ArticleCount, &story.SourceCount, &story.Summary,
		); err != nil {
			return nil, fmt.Errorf("scan newsletter event: %w", err)
		}
		story.Summary = truncateText(story.Summary, 620)
		story.URL = strings.TrimRight(baseURL, "/") + "/e/" + story.ID
		stories = append(stories, story)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("select newsletter events: %w", err)
	}
	return stories, nil
}

func (store *Store) singleArticleStories(ctx context.Context, start, end time.Time, baseURL string) ([]Story, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT article.id::text, article.headline, COALESCE(category.name_si, 'වෙනත්'),
		       article.published_at, source.name,
		       COALESCE(NULLIF(document.summary_text, ''), NULLIF(article.description, ''), article.headline)
		FROM articles article
		JOIN sources source ON source.id = article.source_id
		JOIN rights_profiles rights ON rights.id = article.rights_profile_id
		LEFT JOIN categories category ON category.id = article.category_id
		LEFT JOIN article_analysis_documents document ON document.article_id = article.id
		WHERE article.public_status = 'published'
		  AND article.event_id IS NULL
		  AND article.published_at >= $1 AND article.published_at < $2
		  AND source.active
		  AND rights.mode NOT IN ('disabled', 'internal_verification')
		  AND (rights.expires_at IS NULL OR rights.expires_at > clock_timestamp())
		  AND (category.id IS NULL OR NOT category.held)
		ORDER BY article.published_at DESC, article.id DESC
		LIMIT 20
	`, start, end)
	if err != nil {
		return nil, fmt.Errorf("select newsletter single-source articles: %w", err)
	}
	defer rows.Close()
	stories := make([]Story, 0)
	for rows.Next() {
		var story Story
		var source string
		story.Kind = "article"
		story.ArticleCount = 1
		story.SourceCount = 1
		if err := rows.Scan(&story.ID, &story.Title, &story.Category, &story.LatestAt, &source, &story.Summary); err != nil {
			return nil, fmt.Errorf("scan newsletter single-source article: %w", err)
		}
		story.Summary = truncateText(story.Summary, 460)
		story.Summary = strings.TrimSpace(story.Summary) + " — " + source
		story.URL = strings.TrimRight(baseURL, "/") + "/a/" + story.ID
		stories = append(stories, story)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("select newsletter single-source articles: %w", err)
	}
	return stories, nil
}

func truncateText(value string, maximum int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:maximum-1])) + "…"
}
