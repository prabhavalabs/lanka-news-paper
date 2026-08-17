package ingest

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcdole/gofeed"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/classify"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/cluster"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/normalize"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/sinhala"
)

type Poller struct {
	pool     *pgxpool.Pool
	logger   *slog.Logger
	client   *http.Client
	clusters *cluster.Store
	llm      *llm.Gateway
}

func NewPoller(pool *pgxpool.Pool, logger *slog.Logger, clusters *cluster.Store, gateway *llm.Gateway) *Poller {
	return &Poller{
		pool:     pool,
		logger:   logger,
		clusters: clusters,
		llm:      gateway,
		client:   &http.Client{Timeout: 20 * time.Second, CheckRedirect: limitRedirects},
	}
}

func limitRedirects(req *http.Request, via []*http.Request) error {
	if len(via) >= 3 {
		return fmt.Errorf("too many redirects")
	}
	if via[0].URL.Host != req.URL.Host {
		return fmt.Errorf("redirect to unapproved host")
	}
	return nil
}

func (poller *Poller) PollAll(ctx context.Context) error {
	_, _ = poller.pool.Exec(ctx, `
		UPDATE source_endpoints
		SET health_state = 'stale'
		WHERE NOT paused AND health_state = 'healthy' AND last_success_at IS NOT NULL
		  AND last_success_at < clock_timestamp() - make_interval(secs => polling_interval_seconds * 3)
	`)
	rows, err := poller.pool.Query(ctx, `
		SELECT DISTINCT ON (e.id)
		       e.id::text, e.source_id::text, e.endpoint_type, e.url, COALESCE(e.etag, ''), COALESCE(e.last_modified, ''),
		       r.id::text, r.mode, COALESCE(s.website, ''), s.active
		FROM source_endpoints e
		JOIN sources s ON s.id = e.source_id
		JOIN rights_profiles r ON r.endpoint_id = e.id
		WHERE NOT e.paused
		  AND e.endpoint_type IN ('rss', 'atom', 'rest_api')
		  AND r.mode NOT IN ('disabled', 'internal_verification')
		  AND (r.expires_at IS NULL OR r.expires_at > clock_timestamp())
		  AND (e.backoff_until IS NULL OR e.backoff_until < clock_timestamp())
		  AND (e.last_success_at IS NULL
		       OR e.last_success_at + make_interval(secs => e.polling_interval_seconds) < clock_timestamp())
		ORDER BY e.id, r.version DESC
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type target struct {
		endpointID, sourceID, endpointType, rawURL, etag, lastModified, rightsID, mode, website string
		active                                                                                  bool
	}
	var targets []target
	for rows.Next() {
		var item target
		if err := rows.Scan(&item.endpointID, &item.sourceID, &item.endpointType, &item.rawURL, &item.etag, &item.lastModified, &item.rightsID, &item.mode, &item.website, &item.active); err != nil {
			return err
		}
		targets = append(targets, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range targets {
		if err := poller.pollOne(ctx, item.endpointID, item.sourceID, item.endpointType, item.rawURL, item.etag, item.lastModified, item.rightsID, item.mode, item.website, item.active); err != nil {
			poller.logger.Error("poll failed", "endpoint", item.endpointID, "error", err)
		}
	}
	return nil
}

func (poller *Poller) pollOne(ctx context.Context, endpointID, sourceID, endpointType, rawURL, etag, lastModified, rightsID, mode, website string, sourceActive bool) error {
	startedAt := time.Now().UTC()
	if !strings.HasPrefix(rawURL, "https://") {
		return poller.mark(ctx, endpointID, "failed", "only https endpoints are allowed", "", "")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "SNAP/0.1 (+https://localhost)")
	if etag != "" {
		request.Header.Set("If-None-Match", etag)
	}
	if lastModified != "" {
		request.Header.Set("If-Modified-Since", lastModified)
	}
	response, err := poller.client.Do(request)
	if err != nil {
		return poller.mark(ctx, endpointID, "failed", err.Error(), "", "")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotModified {
		_, _ = poller.pool.Exec(ctx, `
			INSERT INTO ingestion_runs (endpoint_id, started_at, ended_at, status, http_status)
			VALUES ($1, $2, clock_timestamp(), 'ok', $3)
		`, endpointID, startedAt, response.StatusCode)
		return poller.mark(ctx, endpointID, "healthy", "", etag, lastModified)
	}
	if response.StatusCode == http.StatusForbidden || response.StatusCode == http.StatusUnauthorized {
		return poller.mark(ctx, endpointID, "auth_denied", fmt.Sprintf("http %d", response.StatusCode), "", "")
	}
	if response.StatusCode == http.StatusTooManyRequests {
		return poller.mark(ctx, endpointID, "failed", "http 429", "", "")
	}
	if response.StatusCode >= 400 {
		return poller.mark(ctx, endpointID, "failed", fmt.Sprintf("http %d", response.StatusCode), "", "")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 5<<20))
	if err != nil {
		return poller.mark(ctx, endpointID, "failed", err.Error(), "", "")
	}
	feed, err := ParseEndpoint(endpointType, website, body)
	if err != nil {
		sample := string(body)
		if len(sample) > 500 {
			sample = sample[:500]
		}
		_, _ = poller.pool.Exec(ctx, `INSERT INTO quarantine_payloads (endpoint_id, reason, sample) VALUES ($1, $2, $3)`, endpointID, err.Error(), sample)
		return poller.mark(ctx, endpointID, "failed", err.Error(), response.Header.Get("ETag"), response.Header.Get("Last-Modified"))
	}
	newItems := 0
	for _, item := range feed.Items {
		inserted, err := poller.storeItem(ctx, endpointID, sourceID, rightsID, mode, sourceActive, item)
		if inserted {
			newItems++
		}
		if err != nil {
			poller.logger.Error("item failed", "guid", item.GUID, "error", err)
		}
	}
	_, _ = poller.pool.Exec(ctx, `
		INSERT INTO ingestion_runs (endpoint_id, started_at, ended_at, status, http_status, item_count, new_item_count)
		VALUES ($1, $2, clock_timestamp(), 'ok', $3, $4, $5)
	`, endpointID, startedAt, response.StatusCode, len(feed.Items), newItems)
	return poller.mark(ctx, endpointID, "healthy", "", response.Header.Get("ETag"), response.Header.Get("Last-Modified"))
}

func (poller *Poller) storeItem(ctx context.Context, endpointID, sourceID, rightsID, mode string, sourceActive bool, item *gofeed.Item) (bool, error) {
	headline := sinhala.NFC(item.Title)
	if headline == "" || !sinhala.Predominant(headline) {
		return false, nil
	}
	link := item.Link
	if link == "" && len(item.Links) > 0 {
		link = item.Links[0]
	}
	canonical := normalize.CanonicalURL(link)
	itemID := item.GUID
	if itemID == "" {
		itemID = canonical
	}
	publishedAt := time.Now()
	if item.PublishedParsed != nil {
		publishedAt = item.PublishedParsed.UTC()
	} else if item.UpdatedParsed != nil {
		publishedAt = item.UpdatedParsed.UTC()
	}
	fingerprint := normalize.Fingerprint(sourceID, itemID, canonical, headline, publishedAt.UTC().Format(time.RFC3339))
	publisherCat := ""
	if len(item.Categories) > 0 {
		publisherCat = item.Categories[0]
	}
	slug, confidence := classify.From(item.Categories, headline)
	model := "keyword-rules"
	if poller.llm != nil {
		result, _ := poller.llm.Complete(ctx, llm.Request{Task: "classify", Input: headline})
		if result.Text != "" {
			slug, model = result.Text, result.Model
		}
	}
	var near uuid.UUID
	err := poller.pool.QueryRow(ctx, `
		SELECT id FROM articles
		WHERE source_id = $1 AND published_at > clock_timestamp() - interval '6 hours'
		  AND similarity(headline, $2) > 0.85
		LIMIT 1
	`, sourceID, headline).Scan(&near)
	if err == nil {
		return false, nil
	}
	if err != pgx.ErrNoRows {
		return false, err
	}
	status := "held"
	if sourceActive && (mode == "discovery_only" || mode == "licensed_excerpt" || mode == "licensed_media" || mode == "full_syndication") {
		status = "published"
	}
	author := ""
	if item.Author != nil {
		author = item.Author.Name
	}
	var articleID uuid.UUID
	var inserted bool
	err = poller.pool.QueryRow(ctx, `
		INSERT INTO articles (
			source_id, endpoint_id, rights_profile_id, source_item_id, original_url, canonical_url,
			headline, original_headline, description, fingerprint, published_at, public_status,
			publisher_category, category_id, classify_confidence, classify_model, author
		)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, c.id, $14, $15, $16
		FROM categories c WHERE c.slug = $17
		ON CONFLICT (source_id, source_item_id) DO UPDATE SET
			headline = EXCLUDED.headline,
			canonical_url = EXCLUDED.canonical_url,
			fingerprint = EXCLUDED.fingerprint
		WHERE articles.headline IS DISTINCT FROM EXCLUDED.headline
		RETURNING id, (xmax = 0)::boolean
	`, sourceID, endpointID, rightsID, itemID, link, canonical, headline, item.Title, item.Description, fingerprint, publishedAt, status, publisherCat, confidence, model, author, slug).Scan(&articleID, &inserted)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	if !inserted {
		_, _ = poller.pool.Exec(ctx, `
			INSERT INTO article_versions (article_id, version, changed_fields)
			SELECT $1, COALESCE((SELECT max(version) FROM article_versions WHERE article_id = $1), 0) + 1, '{"headline":true}'::jsonb
		`, articleID)
		return false, nil
	}
	if status == "published" && poller.clusters != nil {
		return true, poller.clusters.Attach(ctx, articleID.String(), headline, publishedAt)
	}
	return true, nil
}

func (poller *Poller) mark(ctx context.Context, endpointID, state, detail, etag, lastModified string) error {
	_, err := poller.pool.Exec(ctx, `
		UPDATE source_endpoints
		SET health_state = $2,
		    last_error = NULLIF($3, ''),
		    etag = COALESCE(NULLIF($4, ''), etag),
		    last_modified = COALESCE(NULLIF($5, ''), last_modified),
		    last_success_at = CASE WHEN $2 = 'healthy' THEN clock_timestamp() ELSE last_success_at END,
		    paused = CASE WHEN $2 = 'auth_denied' THEN true ELSE paused END,
		    consecutive_failures = CASE WHEN $2 = 'healthy' THEN 0 ELSE consecutive_failures + 1 END,
		    backoff_until = CASE
		      WHEN $2 = 'healthy' THEN NULL
		      WHEN $2 = 'auth_denied' THEN NULL
		      ELSE clock_timestamp() + make_interval(secs => LEAST(3600, 60 * power(2, LEAST(consecutive_failures, 6))::int))
		           + (random() * interval '20 seconds')
		    END
		WHERE id = $1
	`, endpointID, state, detail, etag, lastModified)
	return err
}

func (poller *Poller) PollEndpoint(ctx context.Context, endpointID string) error {
	var sourceID, endpointType, rawURL, etag, lastModified, rightsID, mode, website string
	var active bool
	err := poller.pool.QueryRow(ctx, `
		SELECT e.source_id::text, e.endpoint_type, e.url, COALESCE(e.etag, ''), COALESCE(e.last_modified, ''),
		       r.id::text, r.mode, COALESCE(s.website, ''), s.active
		FROM source_endpoints e
		JOIN sources s ON s.id = e.source_id
		JOIN rights_profiles r ON r.endpoint_id = e.id
		WHERE e.id = $1
		ORDER BY r.version DESC
		LIMIT 1
	`, endpointID).Scan(&sourceID, &endpointType, &rawURL, &etag, &lastModified, &rightsID, &mode, &website, &active)
	if err != nil {
		return err
	}
	return poller.pollOne(ctx, endpointID, sourceID, endpointType, rawURL, etag, lastModified, rightsID, mode, website, active)
}
