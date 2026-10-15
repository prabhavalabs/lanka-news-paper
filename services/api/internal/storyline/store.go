package storyline

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalid = errors.New("invalid storyline request")
var ErrNotFound = errors.New("storyline not found")
var ErrConflict = errors.New("event already belongs to a storyline")

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

type Reference struct {
	ID       string `json:"id"`
	Slug     string `json:"slug"`
	Title    string `json:"title_si"`
	Trending bool   `json:"trending"`
	Score    int    `json:"trend_score"`
}
type Event struct {
	ID           string    `json:"id"`
	Title        string    `json:"display_title"`
	Time         time.Time `json:"time"`
	SourceCount  int       `json:"source_count"`
	ArticleCount int       `json:"article_count"`
	Summary      *string   `json:"summary"`
}
type Storyline struct {
	Reference
	Summary         string     `json:"summary_si"`
	Status          string     `json:"status"`
	Pinned          bool       `json:"pinned"`
	Hidden          bool       `json:"hidden"`
	SummaryLocked   bool       `json:"summary_locked"`
	ArchivedByAdmin bool       `json:"archived_by_admin"`
	TitleLocked     bool       `json:"title_locked"`
	Metrics         Metrics    `json:"metrics"`
	FirstSeenAt     time.Time  `json:"first_seen_at"`
	LastActivityAt  time.Time  `json:"last_activity_at"`
	TrendingSince   *time.Time `json:"trending_since"`
	LatestEvent     *Event     `json:"latest_event"`
	EventCount      int        `json:"event_count"`
	SourceCount     int        `json:"source_count"`
	ArticleCount    int        `json:"article_count"`
	Events          []Event    `json:"events,omitempty"`
}

type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func groupingInput(ctx context.Context, tx queryer, now time.Time, window time.Duration) ([]InputEvent, []InputStoryline, []BurstTerm, error) {
	rows, err := tx.Query(ctx, `SELECT e.id::text,e.display_title,COALESCE(c.slug,''),count(a.id)::int,count(DISTINCT a.source_id)::int,min(a.published_at),max(a.published_at)
 FROM event_clusters e JOIN (`+publicMembers+`) a ON a.event_id=e.id
 LEFT JOIN categories c ON c.id=e.category_id
 WHERE e.last_update_at >= $1 AND e.last_update_at <= $2 AND e.status<>'merged'
 AND NOT EXISTS(SELECT 1 FROM storyline_events se WHERE se.event_id=e.id)
 AND NOT EXISTS(SELECT 1 FROM storyline_event_exclusions x WHERE x.event_id=e.id)
 GROUP BY e.id,c.slug ORDER BY count(DISTINCT a.source_id) DESC,count(a.id) DESC,e.id LIMIT 80`, now.Add(-window), now)
	if err != nil {
		return nil, nil, nil, err
	}
	events := make([]InputEvent, 0)
	for rows.Next() {
		var e InputEvent
		if err = rows.Scan(&e.ID, &e.Title, &e.Category, &e.Articles, &e.Sources, &e.First, &e.Last); err != nil {
			rows.Close()
			return nil, nil, nil, err
		}
		events = append(events, e)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, nil, nil, err
	}
	rows, err = tx.Query(ctx, `SELECT s.id::text,s.title_si,ARRAY(SELECT e.display_title FROM storyline_events se JOIN event_clusters e ON e.id=se.event_id WHERE se.storyline_id=s.id AND EXISTS(SELECT 1 FROM (`+publicMembers+`) a WHERE a.event_id=e.id) ORDER BY e.last_update_at DESC,e.id LIMIT 6)
 FROM storylines s WHERE s.status='active' AND NOT s.hidden AND NOT s.archived_by_admin AND s.last_activity_at >= $1 ORDER BY s.last_activity_at DESC,s.id`, now.Add(-7*24*time.Hour))
	if err != nil {
		return nil, nil, nil, err
	}
	existing := make([]InputStoryline, 0)
	for rows.Next() {
		var s InputStoryline
		if err = rows.Scan(&s.ID, &s.Title, &s.RecentTitles); err != nil {
			rows.Close()
			return nil, nil, nil, err
		}
		existing = append(existing, s)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, nil, nil, err
	}
	rows, err = tx.Query(ctx, `SELECT id::text,headline,published_at FROM (`+publicMembers+`) a WHERE published_at >= $1 AND published_at <= $2`, now.Add(-8*24*time.Hour), now)
	if err != nil {
		return nil, nil, nil, err
	}
	headlines := make([]Headline, 0)
	for rows.Next() {
		var h Headline
		if err = rows.Scan(&h.ID, &h.Text, &h.At); err != nil {
			rows.Close()
			return nil, nil, nil, err
		}
		headlines = append(headlines, h)
	}
	rows.Close()
	return events, existing, BurstTerms(headlines, now), rows.Err()
}

func applyGroups(ctx context.Context, tx pgx.Tx, groups []Group, now time.Time) error {
	for _, g := range groups {
		id := g.StorylineID
		if id == "" {
			// Recheck membership inside the write transaction, including concurrent admin changes.
			var count, sources int
			if err := tx.QueryRow(ctx, `SELECT count(DISTINCT e.id)::int,count(DISTINCT a.source_id)::int FROM event_clusters e JOIN (`+publicMembers+`) a ON a.event_id=e.id WHERE e.id=ANY($1::uuid[]) AND NOT EXISTS(SELECT 1 FROM storyline_events se WHERE se.event_id=e.id)
 AND NOT EXISTS(SELECT 1 FROM storyline_event_exclusions x WHERE x.event_id=e.id)`, g.EventIDs).Scan(&count, &sources); err != nil {
				return err
			}
			if count < 3 && (count < 2 || sources < 6) {
				continue
			}
			id = uuid.NewString()
			if _, err := tx.Exec(ctx, `INSERT INTO storylines(id,slug,title_si,summary_si,first_seen_at,last_activity_at) VALUES($1,$2,$3,$4,$5,$5)`, id, "story-"+id, g.Title, g.Summary, now); err != nil {
				return err
			}
		}
		var added int64
		for _, eventID := range g.EventIDs {
			tag, err := tx.Exec(ctx, `INSERT INTO storyline_events(storyline_id,event_id,added_by) SELECT id,$2,'ai' FROM storylines WHERE id=$1 AND status='active' AND NOT hidden AND NOT archived_by_admin
 AND EXISTS(SELECT 1 FROM (`+publicMembers+`) a WHERE a.event_id=$2)
 AND NOT EXISTS(SELECT 1 FROM storyline_event_exclusions x WHERE x.event_id=$2) ON CONFLICT(event_id) DO NOTHING`, id, eventID)
			if err != nil {
				return err
			}
			added += tag.RowsAffected()
		}
		if added > 0 {
			if _, err := tx.Exec(ctx, `UPDATE storylines SET title_si=CASE WHEN title_locked THEN title_si ELSE $2 END,summary_si=CASE WHEN summary_locked THEN summary_si ELSE $3 END WHERE id=$1`, id, g.Title, g.Summary); err != nil {
				return err
			}
		}
	}
	return nil
}

func rescore(ctx context.Context, tx pgx.Tx, now time.Time, includeIDs ...string) error {
	if includeIDs == nil {
		includeIDs = []string{}
	}
	rows, err := tx.Query(ctx, `SELECT s.id::text,s.pinned,s.hidden,s.status,s.trending,s.archived_by_admin,
 count(a.id) FILTER(WHERE a.published_at >= $1::timestamptz-interval '3 hours')::int,
 count(a.id) FILTER(WHERE a.published_at >= $1::timestamptz-interval '6 hours')::int,
 count(a.id) FILTER(WHERE a.published_at >= $1::timestamptz-interval '24 hours')::int,
 count(DISTINCT a.source_id) FILTER(WHERE a.published_at >= $1::timestamptz-interval '24 hours')::int,
 count(DISTINCT a.event_id) FILTER(WHERE a.published_at >= $1::timestamptz-interval '24 hours')::int,
 count(a.id) FILTER(WHERE a.published_at >= $1::timestamptz-interval '8 days' AND a.published_at < $1::timestamptz-interval '24 hours')::float8/7,
 COALESCE(min(a.published_at),s.first_seen_at),COALESCE(max(a.published_at),s.first_seen_at)
 FROM storylines s LEFT JOIN storyline_events se ON se.storyline_id=s.id
 LEFT JOIN (`+publicMembers+`) a ON a.event_id=se.event_id AND a.published_at <= $1
 WHERE s.status='active' OR s.trending OR NOT s.archived_by_admin OR s.id=ANY($2::uuid[])
 GROUP BY s.id`, now, includeIDs)
	if err != nil {
		return err
	}
	type scored struct {
		id, status                                string
		pinned, hidden, trending, archivedByAdmin bool
		m                                         Metrics
		first, last                               time.Time
	}
	items := []scored{}
	for rows.Next() {
		var i scored
		if err = rows.Scan(&i.id, &i.pinned, &i.hidden, &i.status, &i.trending, &i.archivedByAdmin, &i.m.Articles3h, &i.m.Articles6h, &i.m.Articles24h, &i.m.Sources24h, &i.m.Events24h, &i.m.BaselineDaily, &i.first, &i.last); err != nil {
			rows.Close()
			return err
		}
		items = append(items, i)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, i := range items {
		if !i.archivedByAdmin {
			i.status = "active"
			if !i.pinned && i.last.Before(now.Add(-7*24*time.Hour)) {
				i.status = "archived"
			}
		}
		score, m := Score(i.m, i.last, now)
		trending := i.status == "active" && IsTrending(score, m, i.pinned, i.hidden, i.trending)
		data, _ := json.Marshal(m)
		if _, err = tx.Exec(ctx, `UPDATE storylines SET trend_score=$2,metrics=$3,first_seen_at=$4,last_activity_at=$5,status=$6,
  trending_since=CASE WHEN $7 AND NOT trending THEN $8 WHEN $7 THEN trending_since ELSE NULL END,trending=$7 WHERE id=$1`, i.id, score, data, i.first, i.last, i.status, trending, now); err != nil {
			return err
		}
	}
	return nil
}
