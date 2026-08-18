package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const algorithmVersion = "headline-hybrid-v2"

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

type article struct {
	id, sourceID uuid.UUID
	categoryID   *uuid.UUID
	headline     string
}

type candidate struct {
	eventID, sourceID uuid.UUID
	headline          string
	trigram           float64
	sameCategory      bool
}

func (store *Store) Attach(ctx context.Context, articleID string) error {
	id, err := uuid.Parse(articleID)
	if err != nil {
		return fmt.Errorf("parse article id: %w", err)
	}
	current, err := store.article(ctx, id)
	if err != nil {
		return err
	}
	match, score, overlap, err := store.bestCandidate(ctx, current)
	if err != nil {
		return err
	}

	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	eventID := uuid.New()
	if match != nil {
		eventID = match.eventID
	} else if _, err := tx.Exec(ctx, `
		INSERT INTO event_clusters (
			id, display_title, category_id, confidence, status,
			first_seen_at, last_update_at, algorithm_version
		)
		SELECT $1, headline, category_id, 1, 'open', published_at, published_at, $3
		FROM articles WHERE id = $2
	`, eventID, current.id, algorithmVersion); err != nil {
		return fmt.Errorf("create event: %w", err)
	}

	signals, err := json.Marshal(map[string]any{
		"combined_score": score,
		"token_overlap":  overlap,
		"category_match": match == nil || match.sameCategory,
	})
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO event_articles (event_id, article_id, clustering_score, origin, signals)
		VALUES ($1, $2, $3, 'automatic', $4)
		ON CONFLICT (event_id, article_id) DO UPDATE
		SET clustering_score = EXCLUDED.clustering_score,
		    signals = EXCLUDED.signals,
		    decided_at = clock_timestamp()
	`, eventID, current.id, score, signals); err != nil {
		return fmt.Errorf("attach event article: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE articles SET event_id = $1 WHERE id = $2`, eventID, current.id); err != nil {
		return fmt.Errorf("link article event: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE event_clusters ec
		SET first_seen_at = stats.first_seen,
		    last_update_at = stats.last_seen,
		    confidence = stats.confidence,
		    is_breaking = stats.recent_sources >= 3,
		    algorithm_version = $2
		FROM (
			SELECT min(a.published_at) first_seen,
			       max(a.published_at) last_seen,
			       avg(ea.clustering_score) confidence,
			       count(DISTINCT a.source_id) FILTER (
			         WHERE a.published_at > clock_timestamp() - interval '60 minutes'
			       ) recent_sources
			FROM event_articles ea
			JOIN articles a ON a.id = ea.article_id
			WHERE ea.event_id = $1
		) stats
		WHERE ec.id = $1
	`, eventID, algorithmVersion); err != nil {
		return fmt.Errorf("update event: %w", err)
	}
	return tx.Commit(ctx)
}

func (store *Store) Backfill(ctx context.Context, limit int) error {
	rows, err := store.pool.Query(ctx, `
		SELECT id::text
		FROM articles
		WHERE public_status = 'published' AND event_id IS NULL
		ORDER BY published_at, id
		LIMIT $1
	`, limit)
	if err != nil {
		return err
	}
	ids := make([]string, 0, limit)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := store.Attach(ctx, id); err != nil {
			return fmt.Errorf("cluster article %s: %w", id, err)
		}
	}
	return nil
}

func (store *Store) article(ctx context.Context, id uuid.UUID) (article, error) {
	var item article
	err := store.pool.QueryRow(ctx, `
		SELECT id, source_id, category_id, headline
		FROM articles
		WHERE id = $1 AND public_status = 'published'
	`, id).Scan(&item.id, &item.sourceID, &item.categoryID, &item.headline)
	if err != nil {
		return article{}, fmt.Errorf("load article: %w", err)
	}
	return item, nil
}

func (store *Store) bestCandidate(ctx context.Context, current article) (*candidate, float64, float64, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT a.event_id, a.source_id, a.headline,
		       similarity(a.headline, $2),
		       a.category_id IS NOT DISTINCT FROM $3::uuid
		FROM articles a
		JOIN event_clusters ec ON ec.id = a.event_id
		JOIN articles current ON current.id = $1
		WHERE a.public_status = 'published'
		  AND a.source_id <> current.source_id
		  AND NOT ec.locked
		  AND a.published_at BETWEEN current.published_at - interval '36 hours'
		                         AND current.published_at + interval '36 hours'
		  AND similarity(a.headline, $2) >= 0.25
		ORDER BY similarity(a.headline, $2) DESC
		LIMIT 30
	`, current.id, current.headline, current.categoryID)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()

	var best *candidate
	bestScore, bestOverlap := 0.0, 0.0
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.eventID, &item.sourceID, &item.headline, &item.trigram, &item.sameCategory); err != nil {
			return nil, 0, 0, err
		}
		score, overlap, matches := similarity(current.headline, item.headline, item.trigram, item.sameCategory)
		if matches && score > bestScore {
			copy := item
			best, bestScore, bestOverlap = &copy, score, overlap
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, 0, err
	}
	if best == nil {
		return nil, 1, 1, nil
	}
	return best, bestScore, bestOverlap, nil
}

func similarity(left, right string, trigram float64, sameCategory bool) (float64, float64, bool) {
	overlap := tokenOverlap(left, right)
	score := trigram*0.72 + overlap*0.28
	if sameCategory {
		score += 0.05
	}
	if score > 1 {
		score = 1
	}
	if !sameCategory && trigram < 0.72 {
		return score, overlap, false
	}
	matches := trigram >= 0.50 || (trigram >= 0.36 && overlap >= 0.35 && score >= 0.50)
	return score, overlap, matches
}

var stopWords = map[string]struct{}{
	"අද": {}, "ඇති": {}, "එක්": {}, "කර": {}, "කළ": {}, "ගැන": {}, "නව": {},
	"ලෙස": {}, "සහ": {}, "සිට": {}, "සඳහා": {}, "වෙයි": {}, "වූ": {}, "වන": {},
	"a": {}, "an": {}, "and": {}, "for": {}, "in": {}, "of": {}, "on": {}, "the": {}, "to": {},
}

func tokenOverlap(left, right string) float64 {
	leftSet, rightSet := tokens(left), tokens(right)
	if len(leftSet) == 0 || len(rightSet) == 0 {
		return 0
	}
	intersection := 0
	for token := range leftSet {
		if _, ok := rightSet[token]; ok {
			intersection++
		}
	}
	union := len(leftSet) + len(rightSet) - intersection
	return float64(intersection) / float64(union)
}

func tokens(value string) map[string]struct{} {
	parts := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r)
	})
	result := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		if _, ignored := stopWords[part]; !ignored {
			result[part] = struct{}{}
		}
	}
	return result
}
