package desk

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/pagination"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/politics"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

type Overview struct {
	Published   int `json:"published"`
	Held        int `json:"held"`
	Quarantined int `json:"quarantined"`
	Complaints  int `json:"complaints"`
	SickFeeds   int `json:"sick_feeds"`
	StaleFeeds  int `json:"stale_feeds"`
	Sources     int `json:"sources"`
}

func (store *Store) Overview(ctx context.Context) (Overview, error) {
	var item Overview
	err := store.pool.QueryRow(ctx, `
		SELECT
		  (SELECT count(*) FROM articles WHERE public_status = 'published'),
		  (SELECT count(*) FROM articles WHERE public_status = 'held'),
		  (SELECT count(*) FROM articles WHERE public_status = 'quarantined'),
		  (SELECT count(*) FROM complaints WHERE status = 'open'),
		  (SELECT count(*) FROM source_endpoints WHERE health_state IN ('failed', 'auth_denied')),
		  (SELECT count(*) FROM source_endpoints
		     WHERE NOT paused AND last_success_at IS NOT NULL
		       AND last_success_at < clock_timestamp() - make_interval(secs => polling_interval_seconds * 3)),
		  (SELECT count(*) FROM sources WHERE archived_at IS NULL)
	`).Scan(&item.Published, &item.Held, &item.Quarantined, &item.Complaints, &item.SickFeeds, &item.StaleFeeds, &item.Sources)
	return item, err
}

// TrendPoint contains one day of dashboard publishing activity.
type TrendPoint struct {
	Date      string `json:"date"`
	Published int    `json:"published"`
	Received  int    `json:"received"`
}

type KnowledgeSummary struct {
	Articles          int `json:"articles"`
	Events            int `json:"events"`
	MultiSourceEvents int `json:"multi_source_events"`
	Sources           int `json:"sources"`
}

type KnowledgeCategory struct {
	Slug     string `json:"slug"`
	NameSI   string `json:"name_si"`
	NameEN   string `json:"name_en"`
	Articles int    `json:"articles"`
	Events   int    `json:"events"`
}

type KnowledgeArticle struct {
	ID          string                    `json:"id"`
	Headline    string                    `json:"headline"`
	SourceID    string                    `json:"source_id"`
	Source      string                    `json:"source"`
	SourceIcon  string                    `json:"source_icon"`
	OriginalURL string                    `json:"original_url"`
	PublishedAt time.Time                 `json:"published_at"`
	Political   *ArticlePoliticalAnalysis `json:"political,omitempty"`
}

type PoliticalMention struct {
	PartySlug  string   `json:"party_slug"`
	Stance     float64  `json:"stance"`
	Confidence float64  `json:"confidence"`
	Terms      []string `json:"terms"`
}

type ArticlePoliticalAnalysis struct {
	Model         string             `json:"model"`
	EconomicFrame float64            `json:"economic_frame"`
	Confidence    float64            `json:"confidence"`
	Mentions      []PoliticalMention `json:"mentions"`
	Relevant      bool               `json:"relevant"`
	Label         string             `json:"label"`
	Rationale     string             `json:"rationale"`
	Evidence      []string           `json:"evidence"`
	ProviderID    string             `json:"provider_id"`
	ProviderModel string             `json:"provider_model"`
}

type PoliticalParty struct {
	Slug             string   `json:"slug"`
	ShortName        string   `json:"short_name"`
	NameEN           string   `json:"name_en"`
	NameSI           string   `json:"name_si"`
	EconomicPosition float64  `json:"economic_position"`
	Confidence       float64  `json:"confidence"`
	Rationale        string   `json:"rationale"`
	EvidenceURLs     []string `json:"evidence_urls"`
}

type SourcePoliticalAnalysis struct {
	SourceID       string  `json:"source_id"`
	Source         string  `json:"source"`
	SourceIcon     string  `json:"source_icon"`
	EconomicFrame  float64 `json:"economic_frame"`
	Confidence     float64 `json:"confidence"`
	RelevantEvents int     `json:"relevant_events"`
	ScoredArticles int     `json:"scored_articles"`
	Qualified      bool    `json:"qualified"`
}

type PoliticalIntelligence struct {
	Axis          string                    `json:"axis"`
	Model         string                    `json:"model"`
	MinimumSample int                       `json:"minimum_sample"`
	Parties       []PoliticalParty          `json:"parties"`
	Sources       []SourcePoliticalAnalysis `json:"sources"`
}

type KnowledgeEvent struct {
	ID               string             `json:"id"`
	Title            string             `json:"title"`
	Category         string             `json:"category"`
	CategoryNameSI   string             `json:"category_name_si"`
	Confidence       float64            `json:"confidence"`
	IsBreaking       bool               `json:"is_breaking"`
	Locked           bool               `json:"locked"`
	AlgorithmVersion string             `json:"algorithm_version"`
	FirstSeenAt      time.Time          `json:"first_seen_at"`
	LastUpdateAt     time.Time          `json:"last_update_at"`
	Articles         []KnowledgeArticle `json:"articles"`
}

type KnowledgeGraph struct {
	GeneratedAt time.Time             `json:"generated_at"`
	Days        int                   `json:"days"`
	Summary     KnowledgeSummary      `json:"summary"`
	Categories  []KnowledgeCategory   `json:"categories"`
	Events      []KnowledgeEvent      `json:"events"`
	Political   PoliticalIntelligence `json:"political"`
}

func (store *Store) KnowledgeGraph(ctx context.Context, start, end time.Time, category string) (KnowledgeGraph, error) {
	result := KnowledgeGraph{
		GeneratedAt: time.Now().UTC(),
		Days:        int((end.Sub(start) + 24*time.Hour - 1) / (24 * time.Hour)),
		Categories:  make([]KnowledgeCategory, 0),
		Events:      make([]KnowledgeEvent, 0),
	}
	if err := store.pool.QueryRow(ctx, `
		WITH scoped AS (
			SELECT a.event_id, a.source_id
			FROM articles a
			LEFT JOIN categories c ON c.id = a.category_id
			WHERE a.public_status = 'published'
			  AND a.event_id IS NOT NULL
			  AND a.published_at >= $1
			  AND a.published_at < $2
			  AND ($3 = '' OR c.slug = $3)
		), events AS (
			SELECT event_id, count(DISTINCT source_id) source_count
			FROM scoped GROUP BY event_id
		)
		SELECT (SELECT count(*) FROM scoped),
		       count(*),
		       count(*) FILTER (WHERE source_count > 1),
		       (SELECT count(DISTINCT source_id) FROM scoped)
		FROM events
	`, start, end, category).Scan(
		&result.Summary.Articles,
		&result.Summary.Events,
		&result.Summary.MultiSourceEvents,
		&result.Summary.Sources,
	); err != nil {
		return KnowledgeGraph{}, fmt.Errorf("summarize knowledge graph: %w", err)
	}

	categoryRows, err := store.pool.Query(ctx, `
		SELECT c.slug, c.name_si, c.name_en, count(*), count(DISTINCT a.event_id)
		FROM articles a
		JOIN categories c ON c.id = a.category_id
		WHERE a.public_status = 'published'
		  AND a.event_id IS NOT NULL
		  AND a.published_at >= $1
		  AND a.published_at < $2
		  AND ($3 = '' OR c.slug = $3)
		GROUP BY c.id, c.slug, c.name_si, c.name_en
		ORDER BY count(*) DESC, c.slug
	`, start, end, category)
	if err != nil {
		return KnowledgeGraph{}, fmt.Errorf("list knowledge categories: %w", err)
	}
	for categoryRows.Next() {
		var item KnowledgeCategory
		if err := categoryRows.Scan(&item.Slug, &item.NameSI, &item.NameEN, &item.Articles, &item.Events); err != nil {
			categoryRows.Close()
			return KnowledgeGraph{}, err
		}
		result.Categories = append(result.Categories, item)
	}
	err = categoryRows.Err()
	categoryRows.Close()
	if err != nil {
		return KnowledgeGraph{}, err
	}

	rows, err := store.pool.Query(ctx, `
		WITH scope AS (
			SELECT a.event_id, max(a.published_at) latest
			FROM articles a
			LEFT JOIN categories c ON c.id = a.category_id
			WHERE a.public_status = 'published'
			  AND a.event_id IS NOT NULL
			  AND a.published_at >= $1
			  AND a.published_at < $2
			  AND ($3 = '' OR c.slug = $3)
			GROUP BY a.event_id
			ORDER BY latest DESC
			LIMIT 150
		)
		SELECT ec.id::text, ec.display_title,
		       COALESCE(c.slug, 'latest'), COALESCE(c.name_si, 'නවතම'),
		       COALESCE(ec.confidence, 0), ec.is_breaking, ec.locked,
		       ec.algorithm_version, ec.first_seen_at, ec.last_update_at,
		       a.id::text, a.headline, s.id::text, s.name, COALESCE(s.icon_url, ''),
		       a.original_url, a.published_at,
		       analysis.model, analysis.economic_frame, analysis.confidence, analysis.mentions,
		       analysis.relevant, analysis.label, analysis.rationale, analysis.evidence,
		       analysis.provider_id, analysis.provider_model
		FROM scope
		JOIN event_clusters ec ON ec.id = scope.event_id
		LEFT JOIN categories c ON c.id = ec.category_id
		JOIN articles a ON a.event_id = ec.id
		JOIN sources s ON s.id = a.source_id
		LEFT JOIN article_political_analysis analysis ON analysis.article_id = a.id
		WHERE a.public_status = 'published'
		  AND a.published_at >= $1
		  AND a.published_at < $2
		ORDER BY scope.latest DESC, a.published_at DESC
	`, start, end, category)
	if err != nil {
		return KnowledgeGraph{}, fmt.Errorf("load knowledge events: %w", err)
	}
	defer rows.Close()
	index := make(map[string]int)
	for rows.Next() {
		var event KnowledgeEvent
		var article KnowledgeArticle
		var politicalModel *string
		var economicFrame, politicalConfidence *float64
		var politicalRelevant *bool
		var politicalLabel, politicalRationale, providerID, providerModel *string
		var politicalMentions, politicalEvidence []byte
		if err := rows.Scan(
			&event.ID, &event.Title, &event.Category, &event.CategoryNameSI,
			&event.Confidence, &event.IsBreaking, &event.Locked,
			&event.AlgorithmVersion, &event.FirstSeenAt, &event.LastUpdateAt,
			&article.ID, &article.Headline, &article.SourceID, &article.Source,
			&article.SourceIcon, &article.OriginalURL, &article.PublishedAt,
			&politicalModel, &economicFrame, &politicalConfidence, &politicalMentions,
			&politicalRelevant, &politicalLabel, &politicalRationale, &politicalEvidence,
			&providerID, &providerModel,
		); err != nil {
			return KnowledgeGraph{}, err
		}
		if politicalModel != nil && economicFrame != nil && politicalConfidence != nil &&
			politicalRelevant != nil && *politicalRelevant {
			analysis := ArticlePoliticalAnalysis{
				Model: *politicalModel, EconomicFrame: *economicFrame,
				Confidence: *politicalConfidence, Relevant: true,
				Mentions: make([]PoliticalMention, 0), Evidence: make([]string, 0),
			}
			if politicalLabel != nil {
				analysis.Label = *politicalLabel
			}
			if politicalRationale != nil {
				analysis.Rationale = *politicalRationale
			}
			if providerID != nil {
				analysis.ProviderID = *providerID
			}
			if providerModel != nil {
				analysis.ProviderModel = *providerModel
			}
			if err := json.Unmarshal(politicalMentions, &analysis.Mentions); err != nil {
				return KnowledgeGraph{}, fmt.Errorf("decode political analysis for article %s: %w", article.ID, err)
			}
			if err := json.Unmarshal(politicalEvidence, &analysis.Evidence); err != nil {
				return KnowledgeGraph{}, fmt.Errorf("decode narration evidence for article %s: %w", article.ID, err)
			}
			article.Political = &analysis
		}
		position, ok := index[event.ID]
		if !ok {
			event.Articles = make([]KnowledgeArticle, 0, 2)
			result.Events = append(result.Events, event)
			position = len(result.Events) - 1
			index[event.ID] = position
		}
		result.Events[position].Articles = append(result.Events[position].Articles, article)
	}
	if err := rows.Err(); err != nil {
		return KnowledgeGraph{}, err
	}
	rows.Close()
	political, err := store.politicalIntelligence(ctx, start, end, category)
	if err != nil {
		return KnowledgeGraph{}, err
	}
	result.Political = political
	return result, nil
}

func (store *Store) politicalIntelligence(ctx context.Context, start, end time.Time, category string) (PoliticalIntelligence, error) {
	const minimumSample = 5
	result := PoliticalIntelligence{
		Axis: "Narration: state-led (-1) to market-led (+1)", Model: politics.Model,
		MinimumSample: minimumSample, Parties: make([]PoliticalParty, 0), Sources: make([]SourcePoliticalAnalysis, 0),
	}
	rows, err := store.pool.Query(ctx, `
		SELECT slug, short_name, name_en, name_si, economic_position, confidence, rationale, evidence_urls
		FROM political_parties WHERE active ORDER BY economic_position, short_name
	`)
	if err != nil {
		return PoliticalIntelligence{}, fmt.Errorf("list political spectrum: %w", err)
	}
	for rows.Next() {
		var item PoliticalParty
		var evidence []byte
		if err := rows.Scan(
			&item.Slug, &item.ShortName, &item.NameEN, &item.NameSI,
			&item.EconomicPosition, &item.Confidence, &item.Rationale, &evidence,
		); err != nil {
			rows.Close()
			return PoliticalIntelligence{}, err
		}
		if err := json.Unmarshal(evidence, &item.EvidenceURLs); err != nil {
			rows.Close()
			return PoliticalIntelligence{}, fmt.Errorf("decode political evidence for %s: %w", item.Slug, err)
		}
		result.Parties = append(result.Parties, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return PoliticalIntelligence{}, err
	}

	rows, err = store.pool.Query(ctx, `
		WITH scoped AS (
			SELECT a.source_id, COALESCE(a.event_id, a.id) sample_id,
			       analysis.economic_frame::float8 economic_frame,
			       analysis.confidence::float8 confidence
			FROM articles a
			LEFT JOIN categories c ON c.id = a.category_id
			JOIN article_political_analysis analysis ON analysis.article_id = a.id
			WHERE a.public_status = 'published'
			  AND a.published_at >= $1
			  AND a.published_at < $2
			  AND ($3 = '' OR c.slug = $3)
			  AND analysis.model = $4
			  AND analysis.relevant
		), samples AS (
			SELECT source_id, sample_id,
			       sum(economic_frame * confidence) / NULLIF(sum(confidence), 0) economic_frame,
			       avg(confidence) confidence
			FROM scoped
			GROUP BY source_id, sample_id
		), aggregate AS (
			SELECT source_id, count(*) relevant_events,
			       count(*) FILTER (WHERE confidence >= 0.6) scored_articles,
			       COALESCE(
			         sum(economic_frame * confidence) FILTER (WHERE confidence >= 0.6)
			         / NULLIF(sum(confidence) FILTER (WHERE confidence >= 0.6), 0), 0
			       ) raw_frame,
			       COALESCE(avg(confidence) FILTER (WHERE confidence >= 0.6), 0) raw_confidence
			FROM samples GROUP BY source_id
		)
		SELECT s.id::text, s.name, COALESCE(s.icon_url, ''),
		       (aggregate.raw_frame * aggregate.scored_articles / (aggregate.scored_articles + 5.0))::float8,
		       (aggregate.raw_confidence * aggregate.scored_articles / (aggregate.scored_articles + 5.0))::float8,
		       aggregate.relevant_events, aggregate.scored_articles,
		       aggregate.scored_articles >= $5
		FROM aggregate JOIN sources s ON s.id = aggregate.source_id
		ORDER BY aggregate.scored_articles DESC, s.name
	`, start, end, category, politics.Model, minimumSample)
	if err != nil {
		return PoliticalIntelligence{}, fmt.Errorf("aggregate source political framing: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item SourcePoliticalAnalysis
		if err := rows.Scan(
			&item.SourceID, &item.Source, &item.SourceIcon, &item.EconomicFrame,
			&item.Confidence, &item.RelevantEvents, &item.ScoredArticles, &item.Qualified,
		); err != nil {
			return PoliticalIntelligence{}, err
		}
		result.Sources = append(result.Sources, item)
	}
	return result, rows.Err()
}

// Trends returns publishing activity for the requested number of days.
func (store *Store) Trends(ctx context.Context, days int) ([]TrendPoint, error) {
	rows, err := store.pool.Query(ctx, `
		WITH days AS (
		  SELECT generate_series(current_date - ($1::int - 1), current_date, interval '1 day')::date AS day
		), published AS (
		  SELECT published_at::date AS day, count(*) AS total
		  FROM articles
		  WHERE public_status = 'published'
		    AND published_at >= current_date - ($1::int - 1)
		  GROUP BY published_at::date
		), received AS (
		  SELECT received_at::date AS day, count(*) AS total
		  FROM articles
		  WHERE received_at >= current_date - ($1::int - 1)
		  GROUP BY received_at::date
		)
		SELECT to_char(days.day, 'YYYY-MM-DD'),
		       COALESCE(published.total, 0), COALESCE(received.total, 0)
		FROM days
		LEFT JOIN published USING (day)
		LEFT JOIN received USING (day)
		ORDER BY days.day
	`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]TrendPoint, 0, days)
	for rows.Next() {
		var item TrendPoint
		if err := rows.Scan(&item.Date, &item.Published, &item.Received); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type QueueItem struct {
	ID         string    `json:"id"`
	Headline   string    `json:"headline"`
	Status     string    `json:"public_status"`
	Source     string    `json:"source"`
	ReceivedAt time.Time `json:"received_at"`
	Confidence *float64  `json:"confidence"`
	Model      *string   `json:"model"`
	Category   *string   `json:"category"`
}

func (store *Store) Queue(ctx context.Context, params pagination.Params, status string) ([]QueueItem, int, error) {
	var total int
	err := store.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM articles a
		JOIN sources s ON s.id = a.source_id
		WHERE ($1 = '' OR a.headline ILIKE '%' || $1 || '%' OR s.name ILIKE '%' || $1 || '%')
		  AND (($2 = '' AND (a.public_status IN ('held', 'quarantined') OR COALESCE(a.classify_confidence, 1) < 0.45))
		    OR ($2 IN ('held', 'quarantined') AND a.public_status = $2)
		    OR ($2 = 'low_confidence' AND COALESCE(a.classify_confidence, 1) < 0.45))
	`, params.Search, status).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count editorial queue: %w", err)
	}

	rows, err := store.pool.Query(ctx, `
		SELECT a.id::text, a.headline, a.public_status, s.name, a.received_at,
		       a.classify_confidence, a.classify_model, c.slug
		FROM articles a
		JOIN sources s ON s.id = a.source_id
		LEFT JOIN categories c ON c.id = a.category_id
		WHERE ($1 = '' OR a.headline ILIKE '%' || $1 || '%' OR s.name ILIKE '%' || $1 || '%')
		  AND (($2 = '' AND (a.public_status IN ('held', 'quarantined') OR COALESCE(a.classify_confidence, 1) < 0.45))
		    OR ($2 IN ('held', 'quarantined') AND a.public_status = $2)
		    OR ($2 = 'low_confidence' AND COALESCE(a.classify_confidence, 1) < 0.45))
		ORDER BY a.received_at DESC, a.id
		LIMIT $3 OFFSET $4
	`, params.Search, status, params.Limit(), params.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list editorial queue: %w", err)
	}
	defer rows.Close()
	items := make([]QueueItem, 0)
	for rows.Next() {
		var item QueueItem
		if err := rows.Scan(&item.ID, &item.Headline, &item.Status, &item.Source, &item.ReceivedAt, &item.Confidence, &item.Model, &item.Category); err != nil {
			return nil, 0, fmt.Errorf("scan editorial queue item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate editorial queue: %w", err)
	}
	return items, total, nil
}

func (store *Store) SetStatus(ctx context.Context, id, status, actor, reason string) error {
	tag, err := store.pool.Exec(ctx, `UPDATE articles SET public_status = $2 WHERE id = $1`, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return err
	}
	_, err = store.pool.Exec(ctx, `
		INSERT INTO editorial_actions (entity_type, entity_id, action, reason)
		VALUES ('article', $1::uuid, $2, $3)
	`, id, status, reason)
	_ = actor
	return err
}

func (store *Store) SetCategory(ctx context.Context, id, slug string) error {
	_, err := store.pool.Exec(ctx, `
		UPDATE articles
		SET category_id = (SELECT id FROM categories WHERE slug = $2),
		    classify_confidence = 1,
		    classify_model = 'manual'
		WHERE id = $1
	`, id, slug)
	return err
}

func (store *Store) SetNote(ctx context.Context, id, note string) error {
	_, err := store.pool.Exec(ctx, `UPDATE articles SET editorial_note = NULLIF($2, '') WHERE id = $1`, id, note)
	return err
}

type Complaint struct {
	ID      string  `json:"id"`
	Type    string  `json:"entity_type"`
	Entity  string  `json:"entity_id"`
	Reason  string  `json:"reason"`
	Contact *string `json:"contact"`
	Status  string  `json:"status"`
}

func (store *Store) Complaints(ctx context.Context, params pagination.Params, status string) ([]Complaint, int, error) {
	var total int
	err := store.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM complaints
		WHERE ($1 = '' OR reason ILIKE '%' || $1 || '%' OR entity_type ILIKE '%' || $1 || '%'
		       OR COALESCE(requester_contact, '') ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR status = $2)
	`, params.Search, status).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count complaints: %w", err)
	}

	rows, err := store.pool.Query(ctx, `
		SELECT id::text, entity_type, entity_id::text, reason, requester_contact, status
		FROM complaints
		WHERE ($1 = '' OR reason ILIKE '%' || $1 || '%' OR entity_type ILIKE '%' || $1 || '%'
		       OR COALESCE(requester_contact, '') ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR status = $2)
		ORDER BY created_at DESC, id
		LIMIT $3 OFFSET $4
	`, params.Search, status, params.Limit(), params.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list complaints: %w", err)
	}
	defer rows.Close()
	items := make([]Complaint, 0)
	for rows.Next() {
		var item Complaint
		if err := rows.Scan(&item.ID, &item.Type, &item.Entity, &item.Reason, &item.Contact, &item.Status); err != nil {
			return nil, 0, fmt.Errorf("scan complaint: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate complaints: %w", err)
	}
	return items, total, nil
}

func (store *Store) ResolveComplaint(ctx context.Context, id, status, resolution string) error {
	_, err := store.pool.Exec(ctx, `
		UPDATE complaints SET status = $2, resolution = $3, resolved_at = clock_timestamp()
		WHERE id = $1
	`, id, status, resolution)
	return err
}

func (store *Store) HoldCategory(ctx context.Context, slug string, held bool) error {
	_, err := store.pool.Exec(ctx, `UPDATE categories SET held = $2 WHERE slug = $1`, slug, held)
	return err
}

func (store *Store) LockCluster(ctx context.Context, id string, locked bool) error {
	_, err := store.pool.Exec(ctx, `UPDATE event_clusters SET locked = $2 WHERE id = $1`, id, locked)
	return err
}

type Quarantine struct {
	ID       string  `json:"id"`
	Endpoint string  `json:"endpoint_id"`
	Reason   string  `json:"reason"`
	Sample   *string `json:"sample"`
}

func (store *Store) Quarantine(ctx context.Context) ([]Quarantine, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT id::text, endpoint_id::text, reason, sample
		FROM quarantine_payloads ORDER BY created_at DESC LIMIT 50
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Quarantine, 0)
	for rows.Next() {
		var item Quarantine
		if err := rows.Scan(&item.ID, &item.Endpoint, &item.Reason, &item.Sample); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (store *Store) MarkStale(ctx context.Context) error {
	_, err := store.pool.Exec(ctx, `
		UPDATE source_endpoints
		SET health_state = 'stale'
		WHERE NOT paused
		  AND health_state = 'healthy'
		  AND last_success_at IS NOT NULL
		  AND last_success_at < clock_timestamp() - make_interval(secs => polling_interval_seconds * 3)
	`)
	return err
}
