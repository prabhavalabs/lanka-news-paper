package desk

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/pagination"
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
		UPDATE articles SET category_id = (SELECT id FROM categories WHERE slug = $2)
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
