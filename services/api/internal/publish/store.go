package publish

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/politics"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

const articleSelect = `
	SELECT a.id::text, a.headline, s.id::text, s.name, s.source_type,
	       c.slug, c.name_si, a.published_at, a.received_at, a.canonical_url,
	       a.event_id::text, a.editorial_note, NULLIF(document.summary_text, ''),
	       analysis.relevant, analysis.label, analysis.left_probability,
	       analysis.center_probability, analysis.right_probability, analysis.confidence
	FROM articles a
	JOIN sources s ON s.id = a.source_id
	JOIN source_endpoints e ON e.id = a.endpoint_id
	JOIN rights_profiles r ON r.id = a.rights_profile_id
	LEFT JOIN categories c ON c.id = a.category_id
	LEFT JOIN article_analysis_documents document ON document.article_id = a.id
	LEFT JOIN article_political_analysis analysis ON analysis.article_id = a.id
	WHERE a.public_status = 'published'
	  AND s.active
	  AND r.mode NOT IN ('disabled', 'internal_verification')
	  AND (r.expires_at IS NULL OR r.expires_at > clock_timestamp())
	  AND (c.id IS NULL OR NOT c.held)
`

type Filter struct {
	Category   string
	SourceID   string
	Query      string
	SourceType string
	From       string
	To         string
	Cursor     string
	Limit      int
}

func (store *Store) ListPublic(ctx context.Context, limit int) (Page, error) {
	return store.ListFiltered(ctx, Filter{Limit: limit})
}

func (store *Store) list(ctx context.Context, filter Filter, offset int) (Page, error) {
	where := ""
	args := []any{}
	n := 1
	if filter.Category != "" && filter.Category != "latest" {
		where += fmt.Sprintf(" AND c.slug = $%d", n)
		args = append(args, filter.Category)
		n++
	}
	if filter.SourceID != "" {
		where += fmt.Sprintf(" AND s.id = $%d::uuid", n)
		args = append(args, filter.SourceID)
		n++
	}
	if filter.Query != "" {
		where += fmt.Sprintf(" AND (a.headline ILIKE $%d OR s.name ILIKE $%d)", n, n)
		args = append(args, "%"+filter.Query+"%")
		n++
	}
	if filter.SourceType != "" {
		where += fmt.Sprintf(" AND s.source_type = $%d", n)
		args = append(args, filter.SourceType)
		n++
	}
	if filter.From != "" {
		where += fmt.Sprintf(" AND a.published_at >= $%d::date", n)
		args = append(args, filter.From)
		n++
	}
	if filter.To != "" {
		where += fmt.Sprintf(" AND a.published_at < ($%d::date + interval '1 day')", n)
		args = append(args, filter.To)
		n++
	}
	args = append(args, filter.Limit+1, offset)
	rows, err := store.pool.Query(ctx, articleSelect+where+fmt.Sprintf(`
		ORDER BY a.published_at DESC, a.id DESC
		LIMIT $%d OFFSET $%d
	`, n, n+1), args...)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	items := make([]Article, 0)
	for rows.Next() {
		item, err := scanArticle(rows)
		if err != nil {
			return Page{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}
	var next *string
	if len(items) > filter.Limit {
		items = items[:filter.Limit]
		token := strconv.Itoa(offset + filter.Limit)
		next = &token
	}
	return Page{Items: items, NextCursor: next}, nil
}

func (store *Store) ListFiltered(ctx context.Context, filter Filter) (Page, error) {
	if filter.Limit <= 0 || filter.Limit > 50 {
		filter.Limit = 20
	}
	offset := 0
	if filter.Cursor != "" {
		offset, _ = strconv.Atoi(filter.Cursor)
	}
	return store.list(ctx, filter, offset)
}

func (store *Store) Get(ctx context.Context, id string) (Article, error) {
	row := store.pool.QueryRow(ctx, articleSelect+` AND a.id = $1`, id)
	return scanArticle(row)
}

func (store *Store) Categories(ctx context.Context) ([]Category, error) {
	rows, err := store.pool.Query(ctx, `SELECT slug, name_si FROM categories WHERE status = 'active' AND NOT held ORDER BY name_en`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Category, 0)
	for rows.Next() {
		var item Category
		if err := rows.Scan(&item.Slug, &item.NameSI); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (store *Store) Sources(ctx context.Context) ([]Source, error) {
	rows, err := store.pool.Query(ctx, `SELECT id::text, name, source_type, description, COALESCE(website, '') FROM sources WHERE active AND archived_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Source, 0)
	for rows.Next() {
		var item Source
		if err := rows.Scan(&item.ID, &item.Name, &item.Type, &item.Description, &item.Website); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type Event struct {
	ID         string                  `json:"id"`
	Title      string                  `json:"title"`
	IsBreaking bool                    `json:"is_breaking"`
	Articles   []Article               `json:"articles"`
	Analysis   *EventNarrativeAnalysis `json:"analysis,omitempty"`
}

func (store *Store) GetEvent(ctx context.Context, id string) (Event, error) {
	var event Event
	if err := store.pool.QueryRow(ctx, `SELECT id::text, display_title, is_breaking FROM event_clusters WHERE id = $1`, id).Scan(&event.ID, &event.Title, &event.IsBreaking); err != nil {
		return Event{}, err
	}
	rows, err := store.pool.Query(ctx, articleSelect+` AND a.event_id = $1 ORDER BY a.published_at`, id)
	if err != nil {
		return Event{}, err
	}
	defer rows.Close()
	event.Articles = make([]Article, 0)
	for rows.Next() {
		item, err := scanArticle(rows)
		if err != nil {
			return Event{}, err
		}
		event.Articles = append(event.Articles, item)
	}
	if err := rows.Err(); err != nil {
		return Event{}, err
	}
	event.Analysis, err = store.eventNarrativeAnalysis(ctx, id)
	if err != nil {
		return Event{}, err
	}
	return event, nil
}

func (store *Store) eventNarrativeAnalysis(ctx context.Context, eventID string) (*EventNarrativeAnalysis, error) {
	var item EventNarrativeAnalysis
	var spectrum []byte
	err := store.pool.QueryRow(ctx, `
		SELECT summary, article_count, source_count, rated_source_count,
		       left_percentage, center_percentage, right_percentage,
		       source_spectrum, analyzed_at
		FROM event_narrative_analyses WHERE event_id = $1
	`, eventID).Scan(
		&item.Summary, &item.ArticleCount, &item.SourceCount, &item.RatedSourceCount,
		&item.LeftPercentage, &item.CenterPercentage, &item.RightPercentage,
		&spectrum, &item.AnalyzedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(spectrum, &item.SourceSpectrum); err != nil {
		return nil, err
	}
	if item.SourceSpectrum == nil {
		item.SourceSpectrum = []EventSourceSpectrum{}
	}
	return &item, nil
}

func (store *Store) Breaking(ctx context.Context) ([]Event, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT id::text, display_title, is_breaking
		FROM event_clusters
		WHERE is_breaking AND last_update_at > clock_timestamp() - interval '6 hours'
		ORDER BY last_update_at DESC
		LIMIT 5
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Event, 0)
	for rows.Next() {
		var item Event
		if err := rows.Scan(&item.ID, &item.Title, &item.IsBreaking); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type Brief struct {
	Date    string `json:"date"`
	TitleSI string `json:"title_si"`
	BodySI  string `json:"body_si"`
	Model   string `json:"model"`
}

func (store *Store) LatestBrief(ctx context.Context) (Brief, error) {
	var brief Brief
	var model *string
	err := store.pool.QueryRow(ctx, `
		SELECT brief_date::text, title_si, body_si, model
		FROM daily_briefs ORDER BY brief_date DESC LIMIT 1
	`).Scan(&brief.Date, &brief.TitleSI, &brief.BodySI, &model)
	if err != nil {
		return Brief{}, err
	}
	if model != nil {
		brief.Model = *model
	}
	return brief, nil
}

func (store *Store) WriteBrief(ctx context.Context) error {
	rows, err := store.pool.Query(ctx, `
		SELECT c.name_si, a.headline, s.name
		FROM articles a
		JOIN sources s ON s.id = a.source_id
		LEFT JOIN categories c ON c.id = a.category_id
		WHERE a.public_status = 'published' AND a.published_at > clock_timestamp() - interval '24 hours'
		ORDER BY c.name_si, a.published_at DESC
		LIMIT 80
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var b strings.Builder
	current := ""
	for rows.Next() {
		var category, headline, source string
		if err := rows.Scan(&category, &headline, &source); err != nil {
			return err
		}
		if category != current {
			b.WriteString("\n")
			b.WriteString(category)
			b.WriteString("\n")
			current = category
		}
		b.WriteString("• ")
		b.WriteString(headline)
		b.WriteString(" (")
		b.WriteString(source)
		b.WriteString(")\n")
	}
	if b.Len() == 0 {
		b.WriteString("පසුගිය පැය 24 තුළ ප්‍රකාශිත අයිතම නැත.")
	}
	today := time.Now().In(time.FixedZone("Asia/Colombo", 5*3600+30*60)).Format("2006-01-02")
	_, err = store.pool.Exec(ctx, `
		INSERT INTO daily_briefs (brief_date, title_si, body_si, model)
		VALUES ($1::date, 'උදෑසන සංග්‍රහය', $2, 'extractive-list')
		ON CONFLICT (brief_date) DO UPDATE SET body_si = EXCLUDED.body_si
	`, today, b.String())
	return err
}

func (store *Store) FileComplaint(ctx context.Context, entityType, entityID, reason, contact string) error {
	_, err := store.pool.Exec(ctx, `
		INSERT INTO complaints (entity_type, entity_id, reason, requester_contact)
		VALUES ($1, $2::uuid, $3, $4)
	`, entityType, entityID, reason, contact)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (store *Store) GetSource(ctx context.Context, id string) (Source, error) {
	var item Source
	err := store.pool.QueryRow(ctx, `
		SELECT id::text, name, source_type, description, COALESCE(website, '')
		FROM sources WHERE id = $1 AND active AND archived_at IS NULL
	`, id).Scan(&item.ID, &item.Name, &item.Type, &item.Description, &item.Website)
	return item, err
}

func (store *Store) ListEvents(ctx context.Context) ([]Event, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT id::text, display_title, is_breaking
		FROM event_clusters
		WHERE last_update_at > clock_timestamp() - interval '48 hours'
		ORDER BY last_update_at DESC
		LIMIT 20
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Event, 0)
	for rows.Next() {
		var item Event
		if err := rows.Scan(&item.ID, &item.Title, &item.IsBreaking); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (store *Store) KnowledgeGraph(ctx context.Context, start, end time.Time, category, sourceID string) (KnowledgeGraph, error) {
	result := KnowledgeGraph{
		GeneratedAt: time.Now().UTC(),
		Days:        int((end.Sub(start) + 24*time.Hour - 1) / (24 * time.Hour)),
		Categories:  make([]KnowledgeCategory, 0),
		Events:      make([]KnowledgeEvent, 0),
	}
	rows, err := store.pool.Query(ctx, `
		WITH scope AS (
			SELECT a.event_id, max(a.published_at) latest
			FROM articles a
			JOIN sources s ON s.id = a.source_id
			JOIN source_endpoints endpoint ON endpoint.id = a.endpoint_id
			JOIN rights_profiles rights ON rights.id = a.rights_profile_id
			JOIN event_clusters scoped_event ON scoped_event.id = a.event_id
			LEFT JOIN categories c ON c.id = scoped_event.category_id
			WHERE a.public_status = 'published'
			  AND a.event_id IS NOT NULL
			  AND s.active AND s.archived_at IS NULL
			  AND rights.mode NOT IN ('disabled', 'internal_verification')
			  AND (rights.expires_at IS NULL OR rights.expires_at > clock_timestamp())
			  AND (c.id IS NULL OR NOT c.held)
			  AND a.published_at >= $1 AND a.published_at < $2
			  AND ($3 = '' OR c.slug = $3)
			  AND ($4 = '' OR s.id = NULLIF($4, '')::uuid)
			GROUP BY a.event_id
			ORDER BY latest DESC
			LIMIT 150
		)
		SELECT event.id::text, event.display_title,
		       COALESCE(category.slug, 'latest'), COALESCE(category.name_si, 'නවතම'),
		       COALESCE(category.name_en, 'Latest'), event.is_breaking, event.last_update_at,
		       event_analysis.summary, event_analysis.article_count, event_analysis.source_count,
		       event_analysis.rated_source_count, event_analysis.left_percentage,
		       event_analysis.center_percentage, event_analysis.right_percentage,
		       event_analysis.source_spectrum, event_analysis.analyzed_at,
		       article.id::text, article.headline, source.id::text, source.name, article.published_at,
		       analysis.label, analysis.economic_frame, analysis.confidence, analysis.relevant,
		       analysis.left_probability, analysis.center_probability,
		       analysis.right_probability, analysis.axis_version
		FROM scope
		JOIN event_clusters event ON event.id = scope.event_id
		LEFT JOIN categories category ON category.id = event.category_id
		LEFT JOIN event_narrative_analyses event_analysis ON event_analysis.event_id = event.id
		JOIN articles article ON article.event_id = event.id
		JOIN sources source ON source.id = article.source_id
		JOIN source_endpoints endpoint ON endpoint.id = article.endpoint_id
		JOIN rights_profiles rights ON rights.id = article.rights_profile_id
		LEFT JOIN categories article_category ON article_category.id = article.category_id
		LEFT JOIN article_political_analysis analysis
		  ON analysis.article_id = article.id AND analysis.model = $5
		WHERE article.public_status = 'published'
		  AND source.active AND source.archived_at IS NULL
		  AND rights.mode NOT IN ('disabled', 'internal_verification')
		  AND (rights.expires_at IS NULL OR rights.expires_at > clock_timestamp())
		  AND (category.id IS NULL OR NOT category.held)
		  AND (article_category.id IS NULL OR NOT article_category.held)
		  AND article.published_at >= $1 AND article.published_at < $2
		ORDER BY scope.latest DESC, article.published_at DESC
	`, start, end, category, sourceID, politics.Model)
	if err != nil {
		return KnowledgeGraph{}, fmt.Errorf("load public knowledge graph: %w", err)
	}
	defer rows.Close()

	eventIndex := make(map[string]int)
	categoryIndex := make(map[string]int)
	sources := make(map[string]struct{})
	for rows.Next() {
		var event KnowledgeEvent
		var article KnowledgeArticle
		var categoryNameEN string
		var label *string
		var frame, confidence, left, center, right *float64
		var relevant *bool
		var axisVersion *string
		var eventSummary *string
		var eventArticleCount, eventSourceCount, eventRatedSourceCount *int
		var eventLeft, eventCenter, eventRight *float64
		var eventSpectrum []byte
		var eventAnalyzedAt *time.Time
		if err := rows.Scan(
			&event.ID, &event.Title, &event.Category, &event.CategoryNameSI, &categoryNameEN,
			&event.IsBreaking, &event.LastUpdateAt, &eventSummary, &eventArticleCount,
			&eventSourceCount, &eventRatedSourceCount, &eventLeft, &eventCenter, &eventRight,
			&eventSpectrum, &eventAnalyzedAt,
			&article.ID, &article.Headline, &article.SourceID, &article.Source, &article.PublishedAt,
			&label, &frame, &confidence, &relevant, &left, &center, &right, &axisVersion,
		); err != nil {
			return KnowledgeGraph{}, err
		}
		if relevant != nil && *relevant && label != nil && frame != nil && confidence != nil {
			article.Narrative = &KnowledgeNarrative{Label: *label, EconomicFrame: *frame, Confidence: *confidence}
			if left != nil {
				article.Narrative.LeftProbability = *left
			}
			if center != nil {
				article.Narrative.CenterProbability = *center
			}
			if right != nil {
				article.Narrative.RightProbability = *right
			}
			if axisVersion != nil {
				article.Narrative.AxisVersion = *axisVersion
			}
		}
		position, ok := eventIndex[event.ID]
		if !ok {
			event.Articles = make([]KnowledgeArticle, 0, 2)
			if eventSummary != nil && eventArticleCount != nil && eventSourceCount != nil &&
				eventRatedSourceCount != nil && eventLeft != nil && eventCenter != nil &&
				eventRight != nil && eventAnalyzedAt != nil {
				event.Analysis = &EventNarrativeAnalysis{
					Summary: *eventSummary, ArticleCount: *eventArticleCount,
					SourceCount: *eventSourceCount, RatedSourceCount: *eventRatedSourceCount,
					LeftPercentage: *eventLeft, CenterPercentage: *eventCenter,
					RightPercentage: *eventRight, AnalyzedAt: *eventAnalyzedAt,
					SourceSpectrum: make([]EventSourceSpectrum, 0),
				}
				if err := json.Unmarshal(eventSpectrum, &event.Analysis.SourceSpectrum); err != nil {
					return KnowledgeGraph{}, fmt.Errorf("decode event source spectrum: %w", err)
				}
			}
			result.Events = append(result.Events, event)
			position = len(result.Events) - 1
			eventIndex[event.ID] = position
			categoryPosition, exists := categoryIndex[event.Category]
			if !exists {
				result.Categories = append(result.Categories, KnowledgeCategory{
					Slug: event.Category, NameSI: event.CategoryNameSI, NameEN: categoryNameEN,
				})
				categoryPosition = len(result.Categories) - 1
				categoryIndex[event.Category] = categoryPosition
			}
			result.Categories[categoryPosition].Events++
		}
		result.Events[position].Articles = append(result.Events[position].Articles, article)
		result.Categories[categoryIndex[event.Category]].Articles++
		sources[article.SourceID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return KnowledgeGraph{}, err
	}
	result.Summary.Events = len(result.Events)
	result.Summary.Sources = len(sources)
	for _, event := range result.Events {
		result.Summary.Articles += len(event.Articles)
		if sourceCount(event.Articles) > 1 {
			result.Summary.MultiSourceEvents++
		}
	}
	return result, nil
}

func sourceCount(articles []KnowledgeArticle) int {
	items := make(map[string]struct{}, len(articles))
	for _, article := range articles {
		items[article.SourceID] = struct{}{}
	}
	return len(items)
}

func scanArticle(row rowScanner) (Article, error) {
	var item Article
	var slug, nameSI, eventID, note *string
	var summary, label *string
	var relevant *bool
	var left, center, right, confidence *float64
	if err := row.Scan(
		&item.ID, &item.Headline, &item.Source.ID, &item.Source.Name, &item.Source.Type,
		&slug, &nameSI, &item.PublishedAt, &item.ReceivedAt, &item.OriginalURL, &eventID, &note,
		&summary, &relevant, &label, &left, &center, &right, &confidence,
	); err != nil {
		return Article{}, err
	}
	if slug != nil && nameSI != nil {
		item.Category = &Category{Slug: *slug, NameSI: *nameSI}
	}
	item.EventID = eventID
	item.EditorialNote = note
	if summary != nil || relevant != nil {
		item.Analysis = &ArticleNarrativeAnalysis{Label: "unrated", CenterProbability: 1}
		if summary != nil {
			item.Analysis.Summary = *summary
		}
		if relevant != nil {
			item.Analysis.Relevant = *relevant
		}
		if label != nil {
			item.Analysis.Label = *label
		}
		if left != nil {
			item.Analysis.LeftProbability = *left
		}
		if center != nil {
			item.Analysis.CenterProbability = *center
		}
		if right != nil {
			item.Analysis.RightProbability = *right
		}
		if confidence != nil {
			item.Analysis.Confidence = *confidence
		}
	}
	return item, nil
}

func IsNotFound(err error) bool {
	return err == pgx.ErrNoRows
}
