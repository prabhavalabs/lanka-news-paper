package storyline

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/textcheck"
)

// The public timeline uses the same publication, rights, source and category gates as news.
const publicMembers = `SELECT a.*,NULLIF(trim(d.summary_text),'') AS article_summary
 FROM articles a JOIN sources source ON source.id=a.source_id
 JOIN rights_profiles rights ON rights.id=a.rights_profile_id
 LEFT JOIN categories category ON category.id=a.category_id
 LEFT JOIN event_clusters parent_event ON parent_event.id=a.event_id
 LEFT JOIN categories parent_category ON parent_category.id=parent_event.category_id
 LEFT JOIN article_analysis_documents d ON d.article_id=a.id
 WHERE a.public_status='published' AND source.active AND (category.id IS NULL OR NOT category.held)
 AND (parent_category.id IS NULL OR NOT parent_category.held) AND (parent_event.id IS NULL OR parent_event.status<>'merged')
 AND a.published_at<=now() AND rights.mode NOT IN ('disabled','internal_verification') AND (rights.expires_at IS NULL OR rights.expires_at>now())`

const hasPublicMember = `EXISTS(SELECT 1 FROM storyline_events member JOIN (` + publicMembers + `) a ON a.event_id=member.event_id WHERE member.storyline_id=s.id)`

// PublicStoryline deliberately contains no editorial controls or internal scoring metrics.
type PublicStoryline struct {
	Reference
	Summary        string     `json:"summary_si"`
	Status         string     `json:"status"`
	FirstSeenAt    time.Time  `json:"first_seen_at"`
	LastActivityAt time.Time  `json:"last_activity_at"`
	TrendingSince  *time.Time `json:"trending_since"`
	LatestEvent    *Event     `json:"latest_event"`
	EventCount     int        `json:"event_count"`
	SourceCount    int        `json:"source_count"`
	ArticleCount   int        `json:"article_count"`
	Events         []Event    `json:"events,omitempty"`
}

func (s Storyline) Public() PublicStoryline {
	return PublicStoryline{Reference: s.Reference, Summary: s.Summary, Status: s.Status, FirstSeenAt: s.FirstSeenAt,
		LastActivityAt: s.LastActivityAt, TrendingSince: s.TrendingSince, LatestEvent: s.LatestEvent,
		EventCount: s.EventCount, SourceCount: s.SourceCount, ArticleCount: s.ArticleCount, Events: s.Events}
}

func (s *Store) List(ctx context.Context, admin bool) ([]Storyline, error) {
	condition := `s.trending AND s.status='active' AND NOT s.hidden AND ` + hasPublicMember
	limit := 5
	if admin {
		condition = `(s.status='active' OR s.updated_at>=now()-interval '30 days')`
		limit = 100
	}
	return s.load(ctx, condition, limit, "", admin)
}
func (s *Store) Get(ctx context.Context, key string, admin bool) (Storyline, error) {
	condition := `(s.id::text=$1 OR s.slug=$1)`
	if !admin {
		condition += ` AND NOT s.hidden AND ` + hasPublicMember
	}
	items, err := s.load(ctx, condition, 1, key, admin)
	if err != nil {
		return Storyline{}, err
	}
	if len(items) == 0 {
		return Storyline{}, ErrNotFound
	}
	return items[0], nil
}

func (s *Store) load(ctx context.Context, condition string, limit int, key string, admin bool) ([]Storyline, error) {
	// All predicates are internal constants; the optional ID/slug remains a bound parameter.
	args := []any{}
	if key != "" {
		args = append(args, key)
	}
	rows, err := s.pool.Query(ctx, `SELECT to_jsonb(s) FROM storylines s WHERE `+condition+` ORDER BY s.pinned DESC,s.trend_score DESC,s.last_activity_at DESC,s.id LIMIT `+strconv.Itoa(limit), args...)
	if err != nil {
		return nil, err
	}
	items := make([]Storyline, 0)
	for rows.Next() {
		var raw []byte
		var item Storyline
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return nil, err
		}
		if err = json.Unmarshal(raw, &item); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err := s.loadEvents(ctx, items, admin); err != nil {
		return nil, err
	}
	return items, nil
}

// Load all member events in one query, collecting distinct source IDs across events.
func (s *Store) loadEvents(ctx context.Context, items []Storyline, admin bool) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, len(items))
	indices := make(map[string]int, len(items))
	for n := range items {
		ids[n] = items[n].ID
		indices[items[n].ID] = n
		items[n].Events = []Event{}
	}
	members, articleJoin := publicMembers, "JOIN q ON q.event_id=e.id"
	if admin {
		members = `SELECT a.*,NULLIF(trim(d.summary_text),'') AS article_summary FROM articles a LEFT JOIN article_analysis_documents d ON d.article_id=a.id WHERE a.public_status='published'`
		articleJoin = "LEFT JOIN q ON q.event_id=e.id"
	}
	rows, err := s.pool.Query(ctx, `WITH q AS (`+members+`)
 SELECT se.storyline_id::text,e.id::text,e.display_title,COALESCE(max(q.published_at),e.last_update_at),count(DISTINCT q.source_id)::int,count(q.id)::int,
 COALESCE((SELECT COALESCE(NULLIF(report.standfirst,''),report.paragraphs->>0) FROM story_reports report WHERE report.event_id=e.id ORDER BY report.created_at DESC LIMIT 1),NULLIF(trim(analysis.summary),''),(array_agg(q.article_summary ORDER BY q.published_at DESC) FILTER(WHERE q.article_summary IS NOT NULL))[1]),
 array_agg(DISTINCT q.source_id::text) FILTER(WHERE q.source_id IS NOT NULL)
 FROM storyline_events se JOIN event_clusters e ON e.id=se.event_id `+articleJoin+`
 LEFT JOIN event_narrative_analyses analysis ON analysis.event_id=e.id
 WHERE se.storyline_id=ANY($1::uuid[])
 GROUP BY se.storyline_id,e.id,analysis.summary ORDER BY COALESCE(max(q.published_at),e.last_update_at) DESC,e.id`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	sources := make(map[string]map[string]bool, len(items))
	for rows.Next() {
		var id string
		var e Event
		var sourceIDs []string
		if err = rows.Scan(&id, &e.ID, &e.Title, &e.Time, &e.SourceCount, &e.ArticleCount, &e.Summary, &sourceIDs); err != nil {
			return err
		}
		if e.Summary != nil {
			if _, foreign := textcheck.Foreign(*e.Summary); foreign {
				e.Summary = nil
			}
		}
		n := indices[id]
		items[n].Events = append(items[n].Events, e)
		items[n].ArticleCount += e.ArticleCount
		if sources[id] == nil {
			sources[id] = map[string]bool{}
		}
		for _, source := range sourceIDs {
			sources[id][source] = true
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for n := range items {
		items[n].EventCount = len(items[n].Events)
		items[n].SourceCount = len(sources[items[n].ID])
		if len(items[n].Events) > 0 {
			items[n].LatestEvent = &items[n].Events[0]
		}
	}
	return nil
}

type Patch struct {
	Title   *string `json:"title_si"`
	Summary *string `json:"summary_si"`
	Pinned  *bool   `json:"pinned"`
	Hidden  *bool   `json:"hidden"`
	Status  *string `json:"status"`
}

func (s *Store) Update(ctx context.Context, id string, p Patch) (Storyline, error) {
	if _, err := uuid.Parse(id); err != nil {
		return Storyline{}, ErrInvalid
	}
	if p.Title != nil {
		value := strings.TrimSpace(*p.Title)
		p.Title = &value
		if value == "" || utf8.RuneCountInString(value) > 120 {
			return Storyline{}, ErrInvalid
		}
	}
	if p.Summary != nil && utf8.RuneCountInString(*p.Summary) > 400 {
		return Storyline{}, ErrInvalid
	}
	if p.Status != nil && *p.Status != "active" && *p.Status != "archived" {
		return Storyline{}, ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Storyline{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(831704266)`); err != nil {
		return Storyline{}, err
	}
	tag, err := tx.Exec(ctx, `UPDATE storylines SET title_si=COALESCE($2,title_si),title_locked=title_locked OR $2 IS NOT NULL,summary_si=COALESCE($3,summary_si),summary_locked=summary_locked OR $3 IS NOT NULL,archived_by_admin=CASE WHEN $6::text IS NULL THEN archived_by_admin ELSE $6='archived' END,pinned=COALESCE($4,pinned),hidden=COALESCE($5,hidden),status=COALESCE($6,status) WHERE id=$1`, id, p.Title, p.Summary, p.Pinned, p.Hidden, p.Status)
	if err != nil {
		return Storyline{}, err
	}
	if tag.RowsAffected() == 0 {
		return Storyline{}, ErrNotFound
	}
	if err = rescore(ctx, tx, time.Now().UTC(), id); err != nil {
		return Storyline{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Storyline{}, err
	}
	return s.Get(ctx, id, true)
}

func (s *Store) Membership(ctx context.Context, id, eventID string, remove bool) error {
	if _, err := uuid.Parse(id); err != nil {
		return ErrInvalid
	}
	if _, err := uuid.Parse(eventID); err != nil {
		return ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(831704266)`); err != nil {
		return err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM storylines WHERE id=$1) AND EXISTS(SELECT 1 FROM event_clusters WHERE id=$2)`, id, eventID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	if remove {
		tag, err := tx.Exec(ctx, `DELETE FROM storyline_events WHERE storyline_id=$1 AND event_id=$2`, id, eventID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		if _, err = tx.Exec(ctx, `INSERT INTO storyline_event_exclusions(storyline_id,event_id) VALUES($1,$2) ON CONFLICT(event_id) DO NOTHING`, id, eventID); err != nil {
			return err
		}
	} else {
		var owner string
		err = tx.QueryRow(ctx, `SELECT storyline_id::text FROM storyline_events WHERE event_id=$1`, eventID).Scan(&owner)
		if err == nil && owner != id {
			return ErrConflict
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM storyline_event_exclusions WHERE event_id=$1`, eventID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO storyline_events(storyline_id,event_id,added_by) VALUES($1,$2,'admin') ON CONFLICT(event_id) DO NOTHING`, id, eventID); err != nil {
			return err
		}
	}
	if err = rescore(ctx, tx, time.Now().UTC(), id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
