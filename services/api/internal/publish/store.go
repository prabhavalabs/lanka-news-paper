package publish

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (store *Store) ListPublic(ctx context.Context, limit int) (Page, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT a.id::text, a.headline, s.id::text, s.name, s.source_type,
		       a.published_at, a.received_at, a.canonical_url
		FROM articles a
		JOIN sources s ON s.id = a.source_id
		JOIN source_endpoints e ON e.id = a.endpoint_id
		JOIN rights_profiles r ON r.id = a.rights_profile_id
		WHERE a.public_status = 'published'
		  AND s.active
		  AND NOT e.paused
		  AND r.mode NOT IN ('disabled', 'internal_verification')
		  AND (r.expires_at IS NULL OR r.expires_at > clock_timestamp())
		ORDER BY a.published_at DESC, a.id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return Page{}, fmt.Errorf("list public articles: %w", err)
	}
	defer rows.Close()

	items := make([]Article, 0)
	for rows.Next() {
		var item Article
		if err := rows.Scan(
			&item.ID,
			&item.Headline,
			&item.Source.ID,
			&item.Source.Name,
			&item.Source.Type,
			&item.PublishedAt,
			&item.ReceivedAt,
			&item.OriginalURL,
		); err != nil {
			return Page{}, fmt.Errorf("scan public article: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Page{}, fmt.Errorf("iterate public articles: %w", err)
	}
	return Page{Items: items}, nil
}
