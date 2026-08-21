package registry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/ingest"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/pagination"
)

type Source struct {
	ID                    string     `json:"id"`
	Name                  string     `json:"name"`
	LegalName             string     `json:"legal_name"`
	SourceType            string     `json:"source_type"`
	Website               string     `json:"website"`
	IconURL               string     `json:"icon_url"`
	Description           string     `json:"description"`
	Active                bool       `json:"active"`
	PublishedArticleCount int64      `json:"published_article_count"`
	LatestPublishedAt     *time.Time `json:"latest_published_at"`
}

type Endpoint struct {
	ID              string  `json:"id"`
	SourceID        string  `json:"source_id"`
	EndpointType    string  `json:"endpoint_type"`
	URL             string  `json:"url"`
	Paused          bool    `json:"paused"`
	HealthState     string  `json:"health_state"`
	LastError       *string `json:"last_error"`
	LastSuccessAt   *string `json:"last_success_at"`
	IntervalSeconds int     `json:"polling_interval_seconds"`
	Verified        bool    `json:"verified_official"`
	LastLatencyMS   *int    `json:"last_latency_ms"`
	LastItemCount   int     `json:"last_item_count"`
	LastNewItems    int     `json:"last_new_item_count"`
	TotalCaptured   int     `json:"total_captured"`
}

type Rights struct {
	ID          string `json:"id"`
	SourceID    string `json:"source_id"`
	EndpointID  string `json:"endpoint_id"`
	Mode        string `json:"mode"`
	Attribution string `json:"attribution"`
}

type SourceTrendPoint struct {
	Date      string `json:"date"`
	Captured  int    `json:"captured"`
	Published int    `json:"published"`
}

type SourcePerformance struct {
	TotalCaptured int                `json:"total_captured"`
	CapturedToday int                `json:"captured_today"`
	Published     int                `json:"published"`
	LastSuccessAt *string            `json:"last_success_at"`
	Daily         []SourceTrendPoint `json:"daily"`
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (store *Store) ListSources(ctx context.Context, params pagination.Params, sourceType, status string) ([]Source, int, error) {
	var total int
	err := store.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM sources
		WHERE archived_at IS NULL
		  AND ($1 = '' OR name ILIKE '%' || $1 || '%' OR legal_name ILIKE '%' || $1 || '%'
		       OR COALESCE(website, '') ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR source_type = $2)
		  AND ($3 = '' OR ($3 = 'active' AND active) OR ($3 = 'held' AND NOT active))
	`, params.Search, sourceType, status).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count sources: %w", err)
	}

	rows, err := store.pool.Query(ctx, `
		SELECT source.id::text, source.name, source.legal_name, source.source_type,
		       COALESCE(source.website, ''), COALESCE(source.icon_url, ''),
		       COALESCE(source.description, ''), source.active,
		       COALESCE(statistics.published_article_count, 0), statistics.latest_published_at
		FROM sources AS source
		LEFT JOIN source_article_statistics AS statistics ON statistics.source_id = source.id
		WHERE source.archived_at IS NULL
		  AND ($1 = '' OR source.name ILIKE '%' || $1 || '%' OR source.legal_name ILIKE '%' || $1 || '%'
		       OR COALESCE(source.website, '') ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR source.source_type = $2)
		  AND ($3 = '' OR ($3 = 'active' AND source.active) OR ($3 = 'held' AND NOT source.active))
		ORDER BY source.name, source.id
		LIMIT $4 OFFSET $5
	`, params.Search, sourceType, status, params.Limit(), params.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()
	items := make([]Source, 0)
	for rows.Next() {
		var item Source
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.LegalName,
			&item.SourceType,
			&item.Website,
			&item.IconURL,
			&item.Description,
			&item.Active,
			&item.PublishedArticleCount,
			&item.LatestPublishedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan source: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate sources: %w", err)
	}
	return items, total, nil
}

func (store *Store) GetSource(ctx context.Context, id string) (Source, error) {
	var item Source
	err := store.pool.QueryRow(ctx, `
		SELECT source.id::text, source.name, source.legal_name, source.source_type,
		       COALESCE(source.website, ''), COALESCE(source.icon_url, ''),
		       COALESCE(source.description, ''), source.active,
		       COALESCE(statistics.published_article_count, 0), statistics.latest_published_at
		FROM sources AS source
		LEFT JOIN source_article_statistics AS statistics ON statistics.source_id = source.id
		WHERE source.id = $1 AND source.archived_at IS NULL
	`, id).Scan(
		&item.ID,
		&item.Name,
		&item.LegalName,
		&item.SourceType,
		&item.Website,
		&item.IconURL,
		&item.Description,
		&item.Active,
		&item.PublishedArticleCount,
		&item.LatestPublishedAt,
	)
	return item, err
}

func (store *Store) GetSourcePerformance(ctx context.Context, id string, days int) (SourcePerformance, error) {
	var performance SourcePerformance
	err := store.pool.QueryRow(ctx, `
		SELECT count(article.id),
		       count(article.id) FILTER (
		         WHERE timezone('Asia/Colombo', article.received_at)::date = timezone('Asia/Colombo', clock_timestamp())::date
		       ),
		       count(article.id) FILTER (WHERE article.public_status = 'published'),
		       (SELECT max(last_success_at)::text FROM source_endpoints WHERE source_id = source.id)
		FROM sources AS source
		LEFT JOIN articles AS article ON article.source_id = source.id
		WHERE source.id = $1 AND source.archived_at IS NULL
		GROUP BY source.id
	`, id).Scan(
		&performance.TotalCaptured,
		&performance.CapturedToday,
		&performance.Published,
		&performance.LastSuccessAt,
	)
	if err != nil {
		return SourcePerformance{}, err
	}

	rows, err := store.pool.Query(ctx, `
		SELECT day::date::text,
		       count(article.id),
		       count(article.id) FILTER (WHERE article.public_status = 'published')
		FROM generate_series(
		       timezone('Asia/Colombo', clock_timestamp())::date - ($2::int - 1),
		       timezone('Asia/Colombo', clock_timestamp())::date,
		       interval '1 day'
		     ) AS day
		LEFT JOIN articles AS article
		  ON article.source_id = $1
		 AND timezone('Asia/Colombo', article.received_at)::date = day::date
		GROUP BY day
		ORDER BY day
	`, id, days)
	if err != nil {
		return SourcePerformance{}, err
	}
	defer rows.Close()
	performance.Daily = make([]SourceTrendPoint, 0, days)
	for rows.Next() {
		var point SourceTrendPoint
		if err := rows.Scan(&point.Date, &point.Captured, &point.Published); err != nil {
			return SourcePerformance{}, err
		}
		performance.Daily = append(performance.Daily, point)
	}
	return performance, rows.Err()
}

func (store *Store) SetActive(ctx context.Context, id string, active bool) error {
	_, err := store.pool.Exec(ctx, `UPDATE sources SET active = $2 WHERE id = $1 AND archived_at IS NULL`, id, active)
	return err
}

func (store *Store) CreateSource(ctx context.Context, item Source) (Source, error) {
	if err := validateIconURL(item.IconURL); err != nil {
		return item, err
	}
	err := store.pool.QueryRow(ctx, `
		INSERT INTO sources (name, legal_name, source_type, website, icon_url, description, active)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7)
		RETURNING id::text
	`, item.Name, item.LegalName, item.SourceType, item.Website, item.IconURL, item.Description, item.Active).Scan(&item.ID)
	return item, err
}

func (store *Store) ListEndpoints(ctx context.Context, sourceID string, params pagination.Params, health, status string) ([]Endpoint, int, error) {
	var total int
	err := store.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM source_endpoints
		WHERE source_id = $1
		  AND ($2 = '' OR url ILIKE '%' || $2 || '%' OR endpoint_type ILIKE '%' || $2 || '%')
		  AND ($3 = '' OR health_state = $3)
		  AND ($4 = '' OR ($4 = 'paused' AND paused) OR ($4 = 'active' AND NOT paused))
	`, sourceID, params.Search, health, status).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count endpoints: %w", err)
	}

	rows, err := store.pool.Query(ctx, `
		SELECT endpoint.id::text, endpoint.source_id::text, endpoint.endpoint_type, endpoint.url,
		       endpoint.paused, endpoint.health_state, endpoint.last_error, endpoint.last_success_at::text,
		       endpoint.polling_interval_seconds, endpoint.verified_official,
		       NULLIF(round(extract(epoch FROM (last_run.ended_at - last_run.started_at)) * 1000)::int, 0),
		       COALESCE(last_run.item_count, 0), COALESCE(last_run.new_item_count, 0),
		       (SELECT count(*) FROM articles WHERE endpoint_id = endpoint.id)
		FROM source_endpoints AS endpoint
		LEFT JOIN LATERAL (
			SELECT started_at, ended_at, item_count, new_item_count
			FROM ingestion_runs
			WHERE endpoint_id = endpoint.id AND ended_at IS NOT NULL
			ORDER BY ended_at DESC
			LIMIT 1
		) AS last_run ON true
		WHERE endpoint.source_id = $1
		  AND ($2 = '' OR endpoint.url ILIKE '%' || $2 || '%' OR endpoint.endpoint_type ILIKE '%' || $2 || '%')
		  AND ($3 = '' OR endpoint.health_state = $3)
		  AND ($4 = '' OR ($4 = 'paused' AND endpoint.paused) OR ($4 = 'active' AND NOT endpoint.paused))
		ORDER BY endpoint.created_at, endpoint.id
		LIMIT $5 OFFSET $6
	`, sourceID, params.Search, health, status, params.Limit(), params.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list endpoints: %w", err)
	}
	defer rows.Close()
	items := make([]Endpoint, 0)
	for rows.Next() {
		var item Endpoint
		if err := rows.Scan(
			&item.ID,
			&item.SourceID,
			&item.EndpointType,
			&item.URL,
			&item.Paused,
			&item.HealthState,
			&item.LastError,
			&item.LastSuccessAt,
			&item.IntervalSeconds,
			&item.Verified,
			&item.LastLatencyMS,
			&item.LastItemCount,
			&item.LastNewItems,
			&item.TotalCaptured,
		); err != nil {
			return nil, 0, fmt.Errorf("scan endpoint: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate endpoints: %w", err)
	}
	return items, total, nil
}

func (store *Store) CreateEndpoint(ctx context.Context, item Endpoint) (Endpoint, error) {
	if !strings.HasPrefix(item.URL, "https://") {
		return item, fmt.Errorf("endpoint must use https")
	}
	id := uuid.New()
	_, err := store.pool.Exec(ctx, `
		INSERT INTO source_endpoints (id, source_id, endpoint_type, url, paused)
		VALUES ($1, $2, $3, $4, true)
	`, id, item.SourceID, item.EndpointType, item.URL)
	item.ID = id.String()
	item.Paused = true
	return item, err
}

func (store *Store) SetPaused(ctx context.Context, endpointID string, paused bool) error {
	_, err := store.pool.Exec(ctx, `UPDATE source_endpoints SET paused = $2 WHERE id = $1`, endpointID, paused)
	return err
}

func (store *Store) ListRights(ctx context.Context, sourceID string, params pagination.Params, mode string) ([]Rights, int, error) {
	var total int
	err := store.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM rights_profiles
		WHERE source_id = $1
		  AND ($2 = '' OR attribution ILIKE '%' || $2 || '%' OR mode ILIKE '%' || $2 || '%')
		  AND ($3 = '' OR mode = $3)
	`, sourceID, params.Search, mode).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count rights profiles: %w", err)
	}

	rows, err := store.pool.Query(ctx, `
		SELECT id::text, source_id::text, endpoint_id::text, mode, attribution
		FROM rights_profiles
		WHERE source_id = $1
		  AND ($2 = '' OR attribution ILIKE '%' || $2 || '%' OR mode ILIKE '%' || $2 || '%')
		  AND ($3 = '' OR mode = $3)
		ORDER BY version DESC, id
		LIMIT $4 OFFSET $5
	`, sourceID, params.Search, mode, params.Limit(), params.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list rights profiles: %w", err)
	}
	defer rows.Close()
	items := make([]Rights, 0)
	for rows.Next() {
		var item Rights
		if err := rows.Scan(&item.ID, &item.SourceID, &item.EndpointID, &item.Mode, &item.Attribution); err != nil {
			return nil, 0, fmt.Errorf("scan rights profile: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate rights profiles: %w", err)
	}
	return items, total, nil
}

func (store *Store) CreateRights(ctx context.Context, item Rights) (Rights, error) {
	err := store.pool.QueryRow(ctx, `
		INSERT INTO rights_profiles (source_id, endpoint_id, version, mode, attribution, effective_from, approved_by, approved_at)
		SELECT $1, $2, COALESCE(max(version), 0) + 1, $3, $4, clock_timestamp(), 'admin', clock_timestamp()
		FROM rights_profiles
		WHERE source_id = $1 AND endpoint_id = $2
		RETURNING id::text
	`, item.SourceID, item.EndpointID, item.Mode, item.Attribution).Scan(&item.ID)
	return item, err
}

func (store *Store) TestEndpoint(ctx context.Context, endpointID string) (map[string]any, error) {
	var rawURL, endpointType, website string
	if err := store.pool.QueryRow(ctx, `
		SELECT endpoint.url, endpoint.endpoint_type, COALESCE(source.website, '')
		FROM source_endpoints AS endpoint
		JOIN sources AS source ON source.id = endpoint.source_id
		WHERE endpoint.id = $1
	`, endpointID).Scan(&rawURL, &endpointType, &website); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(rawURL, "https://") {
		return nil, fmt.Errorf("endpoint must use https")
	}
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many redirects")
		}
		if via[0].URL.Host != req.URL.Host {
			return fmt.Errorf("redirect to unapproved host")
		}
		return nil
	}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 5<<20))
	parseable := false
	latest := ""
	if feed, err := ingest.ParseEndpoint(endpointType, website, body); err == nil && feed != nil {
		parseable = true
		if len(feed.Items) > 0 && feed.Items[0].PublishedParsed != nil {
			latest = feed.Items[0].PublishedParsed.UTC().Format(time.RFC3339)
		}
	}
	return map[string]any{
		"status":      response.StatusCode,
		"contentType": response.Header.Get("Content-Type"),
		"encoding":    response.Header.Get("Content-Type"),
		"parseable":   parseable,
		"latest":      latest,
		"sample":      string(body[:min(500, len(body))]),
	}, nil
}

func (store *Store) Audit(ctx context.Context, actor string, action string, target string, result string) {
	_, _ = store.pool.Exec(ctx, `
		INSERT INTO audit_logs (action, target_type, target_id, result)
		VALUES ($1, 'source', $2, $3)
	`, action, target, result)
}

func (store *Store) Pool() *pgxpool.Pool { return store.pool }

func (store *Store) UpdateSource(ctx context.Context, item Source) error {
	if err := validateIconURL(item.IconURL); err != nil {
		return err
	}
	_, err := store.pool.Exec(ctx, `
		UPDATE sources SET name = $2, legal_name = $3, source_type = $4, website = $5, icon_url = NULLIF($6, ''), description = $7
		WHERE id = $1 AND archived_at IS NULL
	`, item.ID, item.Name, item.LegalName, item.SourceType, item.Website, item.IconURL, item.Description)
	return err
}

func (store *Store) SetSourceIconURL(ctx context.Context, id, iconURL string) error {
	if err := validateIconURL(iconURL); err != nil {
		return err
	}
	_, err := store.pool.Exec(ctx, `
		UPDATE sources SET icon_url = NULLIF($2, '')
		WHERE id = $1 AND archived_at IS NULL
	`, id, iconURL)
	return err
}

func validateIconURL(value string) error {
	if value == "" {
		return nil
	}
	if strings.HasPrefix(value, "/source-logos/") || strings.HasPrefix(value, "/api/admin/media/source-logos/") {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("icon URL must be a valid HTTPS URL")
	}
	return nil
}

func (store *Store) Archive(ctx context.Context, id string) error {
	_, err := store.pool.Exec(ctx, `
		WITH archived AS (
			UPDATE sources
			SET archived_at = clock_timestamp(), active = false
			WHERE id = $1
			RETURNING id
		)
		UPDATE source_endpoints
		SET paused = true
		WHERE source_id IN (SELECT id FROM archived)
	`, id)
	return err
}

func (store *Store) UpdateEndpoint(ctx context.Context, id string, interval int, verified bool) error {
	if interval < 60 {
		interval = 60
	}
	_, err := store.pool.Exec(ctx, `
		UPDATE source_endpoints SET polling_interval_seconds = $2, verified_official = $3 WHERE id = $1
	`, id, interval, verified)
	return err
}

func ParseUUID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid id")
	}
	return id, nil
}
