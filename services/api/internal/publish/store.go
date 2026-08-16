package publish

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

const articleSelect = `
	SELECT a.id::text, a.headline, s.id::text, s.name, s.source_type,
	       c.slug, c.name_si, a.published_at, a.received_at, a.canonical_url,
	       a.event_id::text, a.editorial_note
	FROM articles a
	JOIN sources s ON s.id = a.source_id
	JOIN source_endpoints e ON e.id = a.endpoint_id
	JOIN rights_profiles r ON r.id = a.rights_profile_id
	LEFT JOIN categories c ON c.id = a.category_id
	WHERE a.public_status = 'published'
	  AND s.active
	  AND r.mode NOT IN ('disabled', 'internal_verification')
	  AND (r.expires_at IS NULL OR r.expires_at > clock_timestamp())
	  AND (c.id IS NULL OR NOT c.held)
`

type Filter struct {
	Category   string
	SourceID   string
	Query      string
	SourceType string
	From       string
	To         string
	Cursor     string
	Limit      int
}

func (store *Store) ListPublic(ctx context.Context, limit int) (Page, error) {
	return store.ListFiltered(ctx, Filter{Limit: limit})
}

func (store *Store) list(ctx context.Context, filter Filter, offset int) (Page, error) {
	where := ""
	args := []any{}
	n := 1
	if filter.Category != "" && filter.Category != "latest" {
		where += fmt.Sprintf(" AND c.slug = $%d", n)
		args = append(args, filter.Category)
		n++
	}
	if filter.SourceID != "" {
		where += fmt.Sprintf(" AND s.id = $%d::uuid", n)
		args = append(args, filter.SourceID)
		n++
	}
	if filter.Query != "" {
		where += fmt.Sprintf(" AND (a.headline ILIKE $%d OR s.name ILIKE $%d)", n, n)
		args = append(args, "%"+filter.Query+"%")
		n++
	}
	if filter.SourceType != "" {
		where += fmt.Sprintf(" AND s.source_type = $%d", n)
		args = append(args, filter.SourceType)
		n++
	}
	if filter.From != "" {
		where += fmt.Sprintf(" AND a.published_at >= $%d::date", n)
		args = append(args, filter.From)
		n++
	}
	if filter.To != "" {
		where += fmt.Sprintf(" AND a.published_at < ($%d::date + interval '1 day')", n)
		args = append(args, filter.To)
		n++
	}
	args = append(args, filter.Limit+1, offset)
	rows, err := store.pool.Query(ctx, articleSelect+where+fmt.Sprintf(`
		ORDER BY a.published_at DESC, a.id DESC
		LIMIT $%d OFFSET $%d
	`, n, n+1), args...)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	items := make([]Article, 0)
	for rows.Next() {
		item, err := scanArticle(rows)
		if err != nil {
			return Page{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}
	var next *string
	if len(items) > filter.Limit {
		items = items[:filter.Limit]
		token := strconv.Itoa(offset + filter.Limit)
		next = &token
	}
	return Page{Items: items, NextCursor: next}, nil
}

func (store *Store) ListFiltered(ctx context.Context, filter Filter) (Page, error) {
	if filter.Limit <= 0 || filter.Limit > 50 {
		filter.Limit = 20
	}
	offset := 0
	if filter.Cursor != "" {
		offset, _ = strconv.Atoi(filter.Cursor)
	}
	return store.list(ctx, filter, offset)
}

func (store *Store) Get(ctx context.Context, id string) (Article, error) {
	row := store.pool.QueryRow(ctx, articleSelect+` AND a.id = $1`, id)
	return scanArticle(row)
}

func (store *Store) Categories(ctx context.Context) ([]Category, error) {
	rows, err := store.pool.Query(ctx, `SELECT slug, name_si FROM categories WHERE status = 'active' AND NOT held ORDER BY name_en`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Category, 0)
	for rows.Next() {
		var item Category
		if err := rows.Scan(&item.Slug, &item.NameSI); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (store *Store) Sources(ctx context.Context) ([]Source, error) {
	rows, err := store.pool.Query(ctx, `SELECT id::text, name, source_type, description, COALESCE(website, '') FROM sources WHERE active AND archived_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Source, 0)
	for rows.Next() {
		var item Source
		if err := rows.Scan(&item.ID, &item.Name, &item.Type, &item.Description, &item.Website); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type Event struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	IsBreaking bool      `json:"is_breaking"`
	Articles   []Article `json:"articles"`
}

func (store *Store) GetEvent(ctx context.Context, id string) (Event, error) {
	var event Event
	if err := store.pool.QueryRow(ctx, `SELECT id::text, display_title, is_breaking FROM event_clusters WHERE id = $1`, id).Scan(&event.ID, &event.Title, &event.IsBreaking); err != nil {
		return Event{}, err
	}
	rows, err := store.pool.Query(ctx, articleSelect+` AND a.event_id = $1 ORDER BY a.published_at`, id)
	if err != nil {
		return Event{}, err
	}
	defer rows.Close()
	event.Articles = make([]Article, 0)
	for rows.Next() {
		item, err := scanArticle(rows)
		if err != nil {
			return Event{}, err
		}
		event.Articles = append(event.Articles, item)
	}
	return event, rows.Err()
}

func (store *Store) Breaking(ctx context.Context) ([]Event, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT id::text, display_title, is_breaking
		FROM event_clusters
		WHERE is_breaking AND last_update_at > clock_timestamp() - interval '6 hours'
		ORDER BY last_update_at DESC
		LIMIT 5
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Event, 0)
	for rows.Next() {
		var item Event
		if err := rows.Scan(&item.ID, &item.Title, &item.IsBreaking); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type Brief struct {
	Date    string `json:"date"`
	TitleSI string `json:"title_si"`
	BodySI  string `json:"body_si"`
	Model   string `json:"model"`
}

func (store *Store) LatestBrief(ctx context.Context) (Brief, error) {
	var brief Brief
	var model *string
	err := store.pool.QueryRow(ctx, `
		SELECT brief_date::text, title_si, body_si, model
		FROM daily_briefs ORDER BY brief_date DESC LIMIT 1
	`).Scan(&brief.Date, &brief.TitleSI, &brief.BodySI, &model)
	if err != nil {
		return Brief{}, err
	}
	if model != nil {
		brief.Model = *model
	}
	return brief, nil
}

func (store *Store) WriteBrief(ctx context.Context) error {
	rows, err := store.pool.Query(ctx, `
		SELECT c.name_si, a.headline, s.name
		FROM articles a
		JOIN sources s ON s.id = a.source_id
		LEFT JOIN categories c ON c.id = a.category_id
		WHERE a.public_status = 'published' AND a.published_at > clock_timestamp() - interval '24 hours'
		ORDER BY c.name_si, a.published_at DESC
		LIMIT 80
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var b strings.Builder
	current := ""
	for rows.Next() {
		var category, headline, source string
		if err := rows.Scan(&category, &headline, &source); err != nil {
			return err
		}
		if category != current {
			b.WriteString("\n")
			b.WriteString(category)
			b.WriteString("\n")
			current = category
		}
		b.WriteString("• ")
		b.WriteString(headline)
		b.WriteString(" (")
		b.WriteString(source)
		b.WriteString(")\n")
	}
	if b.Len() == 0 {
		b.WriteString("පසුගිය පැය 24 තුළ ප්‍රකාශිත අයිතම නැත.")
	}
	today := time.Now().In(time.FixedZone("Asia/Colombo", 5*3600+30*60)).Format("2006-01-02")
	_, err = store.pool.Exec(ctx, `
		INSERT INTO daily_briefs (brief_date, title_si, body_si, model)
		VALUES ($1::date, 'උදෑසන සංග්‍රහය', $2, 'extractive-list')
		ON CONFLICT (brief_date) DO UPDATE SET body_si = EXCLUDED.body_si
	`, today, b.String())
	return err
}

func (store *Store) FileComplaint(ctx context.Context, entityType, entityID, reason, contact string) error {
	_, err := store.pool.Exec(ctx, `
		INSERT INTO complaints (entity_type, entity_id, reason, requester_contact)
		VALUES ($1, $2::uuid, $3, $4)
	`, entityType, entityID, reason, contact)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (store *Store) GetSource(ctx context.Context, id string) (Source, error) {
	var item Source
	err := store.pool.QueryRow(ctx, `
		SELECT id::text, name, source_type, description, COALESCE(website, '')
		FROM sources WHERE id = $1 AND active AND archived_at IS NULL
	`, id).Scan(&item.ID, &item.Name, &item.Type, &item.Description, &item.Website)
	return item, err
}

func (store *Store) ListEvents(ctx context.Context) ([]Event, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT id::text, display_title, is_breaking
		FROM event_clusters
		WHERE last_update_at > clock_timestamp() - interval '48 hours'
		ORDER BY last_update_at DESC
		LIMIT 20
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Event, 0)
	for rows.Next() {
		var item Event
		if err := rows.Scan(&item.ID, &item.Title, &item.IsBreaking); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanArticle(row rowScanner) (Article, error) {
	var item Article
	var slug, nameSI, eventID, note *string
	if err := row.Scan(
		&item.ID, &item.Headline, &item.Source.ID, &item.Source.Name, &item.Source.Type,
		&slug, &nameSI, &item.PublishedAt, &item.ReceivedAt, &item.OriginalURL, &eventID, &note,
	); err != nil {
		return Article{}, err
	}
	if slug != nil && nameSI != nil {
		item.Category = &Category{Slug: *slug, NameSI: *nameSI}
	}
	item.EventID = eventID
	item.EditorialNote = note
	return item, nil
}

func IsNotFound(err error) bool {
	return err == pgx.ErrNoRows
}
