package social

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/media"
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

type scanner interface{ Scan(...any) error }

const settingsColumns = `enabled, auto_publish, max_posts_per_hour, max_posts_per_day, min_gap_minutes,
 quiet_hours_start, quiet_hours_end, min_score, lookback_hours, candidates_per_run,
 include_single_articles, timezone, updated_at, realtime_enabled, min_review_score`

func scanSettings(row scanner) (Settings, error) {
	var s Settings
	err := row.Scan(&s.Enabled, &s.AutoPublish, &s.MaxPostsPerHour, &s.MaxPostsPerDay, &s.MinGapMinutes, &s.QuietHoursStart, &s.QuietHoursEnd, &s.MinScore, &s.LookbackHours, &s.CandidatesPerRun, &s.IncludeSingleArticles, &s.Timezone, &s.UpdatedAt, &s.RealtimeEnabled, &s.MinReviewScore)
	return s, err
}

func validateSettings(s Settings) error {
	if strings.TrimSpace(s.Timezone) == "" {
		return ErrInvalid
	}
	if _, err := time.LoadLocation(s.Timezone); err != nil {
		return ErrInvalid
	}
	if s.MaxPostsPerHour < 1 || s.MaxPostsPerHour > 100 || s.MaxPostsPerDay < 1 || s.MaxPostsPerDay > 1000 || s.MinGapMinutes < 1 || s.MinGapMinutes > 1440 || s.QuietHoursStart < 0 || s.QuietHoursStart > 23 || s.QuietHoursEnd < 0 || s.QuietHoursEnd > 23 || s.MinScore < 0 || s.MinScore > 100 || s.LookbackHours < 1 || s.LookbackHours > 168 || s.CandidatesPerRun < 1 || s.CandidatesPerRun > 30 || s.MinReviewScore < 0 || s.MinReviewScore > 100 {
		return ErrInvalid
	}
	return nil
}

func (store *Store) GetSettings(ctx context.Context) (Settings, error) {
	s, err := scanSettings(store.pool.QueryRow(ctx, `SELECT `+settingsColumns+` FROM social_settings WHERE singleton`))
	if err != nil {
		return s, fmt.Errorf("load social settings: %w", err)
	}
	return s, nil
}

func audit(ctx context.Context, tx pgx.Tx, actor uuid.UUID, action, target string) error {
	_, err := tx.Exec(ctx, `INSERT INTO audit_logs (actor_id,action,target_type,target_id,result) VALUES ($1,$2,'social',$3,'ok')`, actor, action, target)
	return err
}

func (store *Store) UpdateSettings(ctx context.Context, s Settings, actor uuid.UUID) (Settings, error) {
	s.Timezone = strings.TrimSpace(s.Timezone)
	if err := validateSettings(s); err != nil {
		return Settings{}, err
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return s, err
	}
	defer tx.Rollback(ctx)
	result, err := scanSettings(tx.QueryRow(ctx, `UPDATE social_settings SET enabled=$1,auto_publish=$2,max_posts_per_hour=$3,
 max_posts_per_day=$4,min_gap_minutes=$5,quiet_hours_start=$6,quiet_hours_end=$7,min_score=$8,
 lookback_hours=$9,candidates_per_run=$10,include_single_articles=$11,timezone=$12,
 updated_by=$13,realtime_enabled=$14,min_review_score=$15,updated_at=clock_timestamp() WHERE singleton RETURNING `+settingsColumns,
		s.Enabled, s.AutoPublish, s.MaxPostsPerHour, s.MaxPostsPerDay, s.MinGapMinutes, s.QuietHoursStart, s.QuietHoursEnd, s.MinScore, s.LookbackHours, s.CandidatesPerRun, s.IncludeSingleArticles, s.Timezone, actor, s.RealtimeEnabled, s.MinReviewScore))
	if err != nil {
		return s, fmt.Errorf("update social settings: %w", err)
	}
	if err = audit(ctx, tx, actor, "update_social_settings", "settings"); err != nil {
		return s, err
	}
	return result, tx.Commit(ctx)
}

// storylineJSON uses an event expression supplied only by internal query constants.
func storylineJSON(event string) string {
	return `(SELECT jsonb_build_object('id',sl.id,'slug',sl.slug,'title_si',sl.title_si,'trending',sl.trending,'trend_score',sl.trend_score) FROM storyline_events se JOIN storylines sl ON sl.id=se.storyline_id WHERE se.event_id=` + event + ` AND NOT sl.hidden AND sl.status='active')`
}

func storylineReevaluationSQL(event, now string) string {
	return `EXISTS(SELECT 1 FROM storyline_events se JOIN storylines sl ON sl.id=se.storyline_id
 WHERE se.event_id=` + event + ` AND sl.trending AND NOT sl.hidden AND sl.status='active'
 AND se.event_id=(SELECT member.event_id FROM storyline_events member JOIN articles latest ON latest.event_id=member.event_id
 WHERE member.storyline_id=sl.id AND latest.public_status='published' AND latest.published_at<=` + now + `
 GROUP BY member.event_id HAVING max(latest.published_at)>=` + now + `-interval '6 hours'
 ORDER BY max(latest.published_at) DESC,member.event_id LIMIT 1))`
}

var postSelect = `SELECT to_jsonb(p) || jsonb_build_object('subject_title',COALESCE(e.display_title,a.headline,''),
 'category',COALESCE(c.name_si,''),'storyline',` + storylineJSON("p.event_id") + `) FROM social_posts p
 LEFT JOIN event_clusters e ON e.id=p.event_id LEFT JOIN articles a ON a.id=p.article_id
 LEFT JOIN categories c ON c.id=COALESCE(e.category_id,a.category_id) `

func scanPost(row scanner) (Post, error) {
	var data []byte
	if err := row.Scan(&data); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Post{}, ErrNotFound
		}
		return Post{}, err
	}
	var p Post
	if err := json.Unmarshal(data, &p); err != nil {
		return p, err
	}
	if p.ImageKey != nil {
		p.ImageURL = media.URL(*p.ImageKey)
	}
	return p, nil
}

func (store *Store) GetPost(ctx context.Context, id string) (Post, error) {
	if _, err := uuid.Parse(id); err != nil {
		return Post{}, ErrInvalid
	}
	return scanPost(store.pool.QueryRow(ctx, postSelect+`WHERE p.id=$1`, id))
}

func validStatus(status string) bool {
	switch status {
	case "pending_review", "approved", "publishing", "published", "rejected", "skipped", "failed", "needs_attention":
		return true
	}
	return false
}

type postCursor struct {
	At time.Time `json:"at"`
	ID string    `json:"id"`
}

func (store *Store) ListPosts(ctx context.Context, status string, limit int, cursor string) (PostList, error) {
	result := PostList{Items: make([]Post, 0)}
	if status != "" && !validStatus(status) {
		return result, ErrInvalid
	}
	if limit < 1 || limit > 100 {
		return result, ErrInvalid
	}
	var at *time.Time
	var id *string
	if cursor != "" {
		data, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return result, ErrInvalid
		}
		var value postCursor
		if json.Unmarshal(data, &value) != nil || value.At.IsZero() {
			return result, ErrInvalid
		}
		if _, err := uuid.Parse(value.ID); err != nil {
			return result, ErrInvalid
		}
		at = &value.At
		id = &value.ID
	}
	rows, err := store.pool.Query(ctx, postSelect+`WHERE ($1='' OR p.status=$1) AND ($2::timestamptz IS NULL OR (p.created_at,p.id)<($2,$3::uuid)) ORDER BY p.created_at DESC,p.id DESC LIMIT $4`, status, at, id, limit+1)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return result, err
		}
		result.Items = append(result.Items, p)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	if len(result.Items) > limit {
		result.Items = result.Items[:limit]
		last := result.Items[len(result.Items)-1]
		data, _ := json.Marshal(postCursor{At: last.CreatedAt, ID: last.ID})
		result.NextCursor = base64.RawURLEncoding.EncodeToString(data)
	}
	return result, nil
}

func storeError(err error) error {
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) {
		if databaseError.Code == "23505" {
			return ErrConflict
		}
		if databaseError.Code == "23503" || databaseError.Code == "22P02" || databaseError.Code == "23514" {
			return ErrInvalid
		}
	}
	return err
}

func lockPost(ctx context.Context, tx pgx.Tx, id string) (Post, error) {
	if _, err := uuid.Parse(id); err != nil {
		return Post{}, ErrInvalid
	}
	return scanPost(tx.QueryRow(ctx, postSelect+`WHERE p.id=$1 FOR UPDATE OF p`, id))
}

func (store *Store) transition(ctx context.Context, id, action string, actor uuid.UUID, scheduled *time.Time, confirm bool, now time.Time) (Post, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return Post{}, err
	}
	defer tx.Rollback(ctx)
	p, err := lockPost(ctx, tx, id)
	if err != nil {
		return p, err
	}
	status := ""
	switch action {
	case "approve":
		if p.Status != "pending_review" && p.Status != "approved" {
			return p, ErrConflict
		}
		status = "approved"
	case "reject":
		if p.Status != "pending_review" && p.Status != "approved" && p.Status != "failed" {
			return p, ErrConflict
		}
		status = "rejected"
	case "retry":
		if p.Status != "failed" && p.Status != "needs_attention" {
			return p, ErrConflict
		}
		if p.Status == "needs_attention" && !confirm {
			return p, fmt.Errorf("%w: confirm_not_published must be true after checking the Page", ErrInvalid)
		}
		status = "approved"
	default:
		return p, ErrInvalid
	}
	if scheduled == nil && status == "approved" {
		scheduled = &now
	}
	if _, err = tx.Exec(ctx, `UPDATE social_posts SET status=$2,scheduled_for=$3,
 approved_by=CASE WHEN $2='approved' THEN $4 ELSE approved_by END,
 approved_at=CASE WHEN $2='approved' THEN $5 ELSE approved_at END,
 rejected_by=CASE WHEN $2='rejected' THEN $4 ELSE rejected_by END,
 rejected_at=CASE WHEN $2='rejected' THEN $5 ELSE rejected_at END,
 loomcast_post_id=CASE WHEN $2='approved' THEN NULL ELSE loomcast_post_id END,
 handed_off_at=CASE WHEN $2='approved' THEN NULL ELSE handed_off_at END,last_error=NULL WHERE id=$1`, id, status, scheduled, actor, now); err != nil {
		return p, storeError(err)
	}
	if err = audit(ctx, tx, actor, "social_post_"+action, id); err != nil {
		return p, err
	}
	if err = tx.Commit(ctx); err != nil {
		return p, storeError(err)
	}
	return store.GetPost(ctx, id)
}

func publishablePost(ctx context.Context, tx pgx.Tx, p Post, now time.Time) (bool, error) {
	var eligible bool
	err := tx.QueryRow(ctx, `
 WITH permitted AS (
 SELECT DISTINCT article.id,source.id AS source_id,source.name FROM articles article
 JOIN sources source ON source.id=article.source_id
 JOIN rights_profiles rights ON rights.id=article.rights_profile_id
 LEFT JOIN categories category ON category.id=article.category_id
 LEFT JOIN event_clusters event ON event.id=article.event_id
 LEFT JOIN categories event_category ON event_category.id=event.category_id
 WHERE (($1::uuid IS NOT NULL AND article.event_id=$1) OR ($2::uuid IS NOT NULL AND article.id=$2))
 AND article.public_status='published' AND source.active
 AND rights.mode NOT IN ('disabled','internal_verification')
 AND (rights.expires_at IS NULL OR rights.expires_at>$3)
 AND (category.id IS NULL OR NOT category.held)
 AND (event_category.id IS NULL OR NOT event_category.held)
 )
 SELECT count(DISTINCT source_id)>=$4 AND COALESCE(array_agg(DISTINCT name),'{}'::text[]) @> $5::text[]
 AND cardinality(post.evidence_article_ids)>0
 AND COALESCE(array_agg(DISTINCT permitted.id) FILTER (WHERE permitted.id IS NOT NULL),'{}'::uuid[]) @> post.evidence_article_ids
 FROM social_posts post LEFT JOIN permitted ON true WHERE post.id=$6 GROUP BY post.evidence_article_ids`, p.EventID, p.ArticleID, now, p.SourceCount, p.SourceNames, p.ID).Scan(&eligible)
	return eligible, err
}

// autoApprove promotes a reviewed post for automatic publishing. It re-checks, under lock,
// that publishing and auto-publish are still on and that the post's current text passed
// the AI review, so a setting changed during the model calls is always respected.
func (store *Store) autoApprove(ctx context.Context, id string, now time.Time) (bool, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var allowed bool
	if err = tx.QueryRow(ctx, `SELECT enabled AND auto_publish FROM social_settings WHERE singleton FOR SHARE`).Scan(&allowed); err != nil {
		return false, err
	}
	if !allowed {
		return false, nil
	}
	result, err := tx.Exec(ctx, `UPDATE social_posts SET status='approved',scheduled_for=$2,approved_at=$2,hold_reason=NULL
 WHERE id=$1 AND status='pending_review' AND review_status='passed'`, id, now)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() == 1, tx.Commit(ctx)
}
