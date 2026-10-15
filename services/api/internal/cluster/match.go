package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MatchCandidate is an earlier event that an article standing alone may report on. Outlets
// word one incident differently (and give different figures), so headline similarity alone
// misses many of them; the pipeline asks a model to decide among these.
type MatchCandidate struct {
	EventID         string    `json:"-"`
	Headlines       []string  `json:"headlines"`
	Summary         string    `json:"summary"`
	FirstReportedAt time.Time `json:"first_reported_at"`
}

// MatchCandidates returns the event an article is in and the earlier events, from the last
// 36 hours, with the most similar headlines. It returns none unless the article is alone in
// an open event that nothing else (a social post, the front page, a report, a storyline) refers to yet.
// Only earlier events are offered, so two lone reports never join each other both ways.
func (store *Store) MatchCandidates(ctx context.Context, articleID string) (string, []MatchCandidate, error) {
	rows, err := store.pool.Query(ctx, `
		WITH current AS (
		  SELECT a.id, a.event_id, a.headline, a.published_at
		  FROM articles a
		  JOIN event_clusters own ON own.id = a.event_id AND own.status = 'open' AND NOT own.locked
		  WHERE a.id = $1 AND a.public_status = 'published'
		    AND NOT EXISTS (SELECT 1 FROM articles other WHERE other.event_id = a.event_id AND other.id <> a.id)
		    AND NOT EXISTS (SELECT 1 FROM social_posts post WHERE post.event_id = a.event_id)
		    AND NOT EXISTS (SELECT 1 FROM front_page_placements placement WHERE placement.event_id = a.event_id)
		    AND NOT EXISTS (SELECT 1 FROM story_reports report WHERE report.event_id = a.event_id)
		    AND NOT EXISTS (SELECT 1 FROM storyline_events member WHERE member.event_id = a.event_id)
		)
		SELECT current.event_id::text, ec.id::text, ec.first_seen_at,
		       (array_agg(member.headline ORDER BY member.published_at))[1:3],
		       COALESCE((array_agg(document.summary_text ORDER BY member.published_at)
		                 FILTER (WHERE document.summary_text <> ''))[1], '')
		FROM current
		JOIN articles member ON member.public_status = 'published' AND member.event_id <> current.event_id
		  AND member.published_at >= current.published_at - interval '36 hours'
		JOIN event_clusters ec ON ec.id = member.event_id AND ec.status = 'open' AND NOT ec.locked
		  AND ec.first_seen_at <= current.published_at
		LEFT JOIN article_analysis_documents document ON document.article_id = member.id
		GROUP BY current.event_id, ec.id, ec.first_seen_at
		HAVING max(similarity(member.headline, current.headline)) >= 0.08
		ORDER BY max(similarity(member.headline, current.headline)) DESC
		LIMIT 12
	`, articleID)
	if err != nil {
		return "", nil, fmt.Errorf("load event match candidates: %w", err)
	}
	defer rows.Close()
	var own string
	candidates := make([]MatchCandidate, 0)
	for rows.Next() {
		var item MatchCandidate
		if err := rows.Scan(&own, &item.EventID, &item.FirstReportedAt, &item.Headlines, &item.Summary); err != nil {
			return "", nil, err
		}
		item.Summary = truncate(item.Summary, 700)
		candidates = append(candidates, item)
	}
	return own, candidates, rows.Err()
}

// Join moves an article that stands alone in its event into an earlier event that reports the
// same incident, and marks the emptied event merged. It returns false, changing nothing, when
// either event changed since the candidates were read.
func (store *Store) Join(ctx context.Context, articleID, fromEventID, toEventID, reason string) (bool, error) {
	from, err := uuid.Parse(fromEventID)
	if err != nil {
		return false, err
	}
	to, err := uuid.Parse(toEventID)
	if err != nil {
		return false, err
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	// Lock both events in a fixed order so concurrent joins cannot deadlock or cross.
	var open int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM (
		  SELECT id, status, locked FROM event_clusters WHERE id IN ($1, $2) ORDER BY id FOR UPDATE
		) locked_events WHERE status = 'open' AND NOT locked
	`, from, to).Scan(&open); err != nil {
		return false, err
	}
	var alone bool
	if err := tx.QueryRow(ctx, `
		SELECT count(*) = 1 AND bool_and(id = $2) AND NOT EXISTS(SELECT 1 FROM storyline_events WHERE event_id=$1) FROM articles WHERE event_id = $1
	`, from, articleID).Scan(&alone); err != nil {
		return false, err
	}
	if open != 2 || !alone {
		return false, nil
	}
	signals, err := json.Marshal(map[string]any{"ai_match": true, "reason": truncate(reason, 500)})
	if err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM event_articles WHERE event_id = $1 AND article_id = $2`, from, articleID); err != nil {
		return false, fmt.Errorf("move event article: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO event_articles (event_id, article_id, origin, signals) VALUES ($1, $2, 'automatic', $3)
		ON CONFLICT (event_id, article_id) DO NOTHING
	`, to, articleID, signals); err != nil {
		return false, fmt.Errorf("move event article: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE articles SET event_id = $2 WHERE id = $1`, articleID, to); err != nil {
		return false, fmt.Errorf("link article event: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE event_clusters SET status = 'merged' WHERE id = $1`, from); err != nil {
		return false, fmt.Errorf("retire merged event: %w", err)
	}
	if err := refresh(ctx, tx, to); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
