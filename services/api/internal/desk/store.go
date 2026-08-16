package desk

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
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

type QueueItem struct {
	ID         string   `json:"id"`
	Headline   string   `json:"headline"`
	Status     string   `json:"public_status"`
	Source     string   `json:"source"`
	Confidence *float64 `json:"confidence"`
	Model      *string  `json:"model"`
	Category   *string  `json:"category"`
}

func (store *Store) Queue(ctx context.Context) ([]QueueItem, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT a.id::text, a.headline, a.public_status, s.name,
		       a.classify_confidence, a.classify_model, c.slug
		FROM articles a
		JOIN sources s ON s.id = a.source_id
		LEFT JOIN categories c ON c.id = a.category_id
		WHERE a.public_status IN ('held', 'quarantined')
		   OR COALESCE(a.classify_confidence, 1) < 0.45
		ORDER BY a.received_at DESC
		LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]QueueItem, 0)
	for rows.Next() {
		var item QueueItem
		if err := rows.Scan(&item.ID, &item.Headline, &item.Status, &item.Source, &item.Confidence, &item.Model, &item.Category); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
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

func (store *Store) Complaints(ctx context.Context) ([]Complaint, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT id::text, entity_type, entity_id::text, reason, requester_contact, status
		FROM complaints ORDER BY created_at DESC LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Complaint, 0)
	for rows.Next() {
		var item Complaint
		if err := rows.Scan(&item.ID, &item.Type, &item.Entity, &item.Reason, &item.Contact, &item.Status); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
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
