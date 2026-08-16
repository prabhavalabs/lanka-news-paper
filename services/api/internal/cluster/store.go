package cluster

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (store *Store) Attach(ctx context.Context, articleID, headline string, publishedAt time.Time) error {
	var existingID uuid.UUID
	var existingSource uuid.UUID
	err := store.pool.QueryRow(ctx, `
		SELECT a.id, a.source_id
		FROM articles a
		WHERE a.public_status = 'published'
		  AND a.id <> $1
		  AND a.published_at > clock_timestamp() - interval '18 hours'
		  AND similarity(a.headline, $2) > 0.38
		ORDER BY similarity(a.headline, $2) DESC
		LIMIT 1
	`, articleID, headline).Scan(&existingID, &existingSource)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}

	var clusterID uuid.UUID
	var currentSource uuid.UUID
	if err := store.pool.QueryRow(ctx, `SELECT source_id FROM articles WHERE id = $1`, articleID).Scan(&currentSource); err != nil {
		return err
	}
	if currentSource == existingSource {
		return nil
	}

	err = store.pool.QueryRow(ctx, `SELECT event_id FROM articles WHERE id = $1 AND event_id IS NOT NULL`, existingID).Scan(&clusterID)
	if err == pgx.ErrNoRows {
		clusterID = uuid.New()
		_, err = store.pool.Exec(ctx, `
			INSERT INTO event_clusters (id, display_title, status, first_seen_at, last_update_at)
			VALUES ($1, $2, 'open', $3, $3)
		`, clusterID, headline, publishedAt)
		if err != nil {
			return err
		}
		_, _ = store.pool.Exec(ctx, `UPDATE articles SET event_id = $1 WHERE id = $2`, clusterID, existingID)
		_, _ = store.pool.Exec(ctx, `INSERT INTO event_articles (event_id, article_id, origin) VALUES ($1, $2, 'automatic') ON CONFLICT DO NOTHING`, clusterID, existingID)
	} else if err != nil {
		return err
	} else {
		var locked bool
		if err := store.pool.QueryRow(ctx, `SELECT locked FROM event_clusters WHERE id = $1`, clusterID).Scan(&locked); err != nil {
			return err
		}
		if locked {
			return nil
		}
	}

	_, _ = store.pool.Exec(ctx, `UPDATE articles SET event_id = $1 WHERE id = $2`, clusterID, articleID)
	_, _ = store.pool.Exec(ctx, `INSERT INTO event_articles (event_id, article_id, origin) VALUES ($1, $2, 'automatic') ON CONFLICT DO NOTHING`, clusterID, articleID)
	_, _ = store.pool.Exec(ctx, `
		UPDATE event_clusters SET last_update_at = clock_timestamp(),
		  is_breaking = (
		    SELECT COUNT(DISTINCT source_id) >= 3
		    FROM articles WHERE event_id = $1 AND published_at > clock_timestamp() - interval '60 minutes'
		  )
		WHERE id = $1
	`, clusterID)
	return nil
}
