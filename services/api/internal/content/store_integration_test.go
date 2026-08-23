package content

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestCaptureStructuredIntegration(t *testing.T) {
	databaseURL := os.Getenv("SNAP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SNAP_TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	sourceID, endpointID, rightsID, articleID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	name := "Content integration " + sourceID.String()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
		INSERT INTO sources (id, name, legal_name, source_type, website, active)
		VALUES ($1, $2, $2, 'other', 'https://news.example', false)
	`, sourceID, name)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `
		INSERT INTO source_endpoints (id, source_id, endpoint_type, url, paused)
		VALUES ($1, $2, 'rss', 'https://news.example/feed.xml', true)
	`, endpointID, sourceID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `
		INSERT INTO rights_profiles (
			id, source_id, endpoint_id, mode, attribution, effective_from,
			raw_payload_retention_days
		) VALUES ($1, $2, $3, 'full_syndication', 'Test source', clock_timestamp(), 7)
	`, rightsID, sourceID, endpointID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `
		INSERT INTO source_collection_profiles (
			source_id, endpoint_id, version, active, discovery_method, article_method,
			config, min_delay_seconds, max_requests_per_run, max_pages,
			request_timeout_seconds, created_by, activated_at
		) VALUES (
			$1, $2, 1, true, 'rss', 'feed_content',
			'{"discovery_urls":["https://news.example/feed.xml"],"allowed_hosts":["news.example"],"article_url_patterns":[],"exclude_selectors":[],"pagination_mode":"none","user_agent":"SNAPBot/1.0","min_content_characters":100,"minimum_sinhala_ratio":0}'::jsonb,
			5, 10, 1, 10, 'test', clock_timestamp()
		)
	`, sourceID, endpointID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `
		INSERT INTO source_compliance_reviews (
			source_id, version, active, status, robots_url, robots_checked_at,
			robots_allowed, allow_discovery, allow_full_text_storage, reviewed_by
		) VALUES (
			$1, 1, true, 'approved', 'https://news.example/robots.txt',
			clock_timestamp(), true, true, true, 'test'
		)
	`, sourceID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `
		INSERT INTO articles (
			id, source_id, endpoint_id, rights_profile_id, source_item_id,
			original_url, canonical_url, headline, fingerprint, published_at
		) VALUES (
			$1, $2, $3, $4, $5, 'https://news.example/story/1',
			'https://news.example/story/1', 'සිංහල පරීක්ෂණ පුවත', $5,
			clock_timestamp()
		)
	`, articleID, sourceID, endpointID, rightsID, articleID.String())
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM articles WHERE source_id = $1`, sourceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM source_compliance_reviews WHERE source_id = $1`, sourceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM source_collection_profiles WHERE source_id = $1`, sourceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM rights_profiles WHERE source_id = $1`, sourceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM source_endpoints WHERE source_id = $1`, sourceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM sources WHERE id = $1`, sourceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM crawl_domain_leases WHERE host = $1`, sourceID.String()+".example")
	})

	store := NewStore(pool)
	leaseHost := sourceID.String() + ".example"
	require.NoError(t, store.acquireDomainLease(ctx, leaseHost, 30))
	var rateLimit *RateLimitError
	require.ErrorAs(t, store.acquireDomainLease(ctx, leaseHost, 30), &rateLimit)
	require.Equal(t, 30*time.Second, rateLimit.RetryAfter)

	body := `<article><p>` + strings.Repeat("සම්පූර්ණ සිංහල පුවත් අන්තර්ගතය. ", 12) + `</p><script>unsafe()</script></article>`
	stored, err := store.CaptureStructured(ctx, articleID.String(), body, "feed_content")
	require.NoError(t, err)
	require.True(t, stored)

	var captured, method string
	var retention time.Time
	err = pool.QueryRow(ctx, `
		SELECT body_text, acquisition_method, retention_until
		FROM article_contents WHERE article_id = $1 AND current
	`, articleID).Scan(&captured, &method, &retention)
	require.NoError(t, err)
	require.NotContains(t, captured, "unsafe")
	require.Equal(t, "feed_content", method)
	require.WithinDuration(t, time.Now().UTC().Add(7*24*time.Hour), retention, time.Minute)

	stored, err = store.CaptureStructured(ctx, articleID.String(), body, "feed_content")
	require.NoError(t, err)
	require.False(t, stored, fmt.Sprintf("identical content should not create another version for %s", articleID))

	_, err = pool.Exec(ctx, `
		UPDATE article_contents SET retention_until = clock_timestamp() - interval '1 minute'
		WHERE article_id = $1
	`, articleID)
	require.NoError(t, err)
	deleted, err := store.DeleteExpired(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, deleted, int64(1))
	var remains bool
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM article_contents WHERE article_id = $1)
	`, articleID).Scan(&remains))
	require.False(t, remains)

	structuredBackfillBody := `<article><p>` + strings.Repeat("පැරණි සම්පූර්ණ සිංහල පුවත් අන්තර්ගතය. ", 12) + `</p></article>`
	_, err = pool.Exec(ctx, `
		UPDATE articles SET description = $2 WHERE id = $1
	`, articleID, structuredBackfillBody)
	require.NoError(t, err)
	require.NoError(t, store.Enrich(ctx, articleID.String()))
	err = pool.QueryRow(ctx, `
		SELECT body_text, acquisition_method
		FROM article_contents WHERE article_id = $1 AND current
	`, articleID).Scan(&captured, &method)
	require.NoError(t, err)
	require.Contains(t, captured, "පැරණි සම්පූර්ණ")
	require.Equal(t, "feed_content", method)
	_, err = pool.Exec(ctx, `DELETE FROM article_contents WHERE article_id = $1`, articleID)
	require.NoError(t, err)

	candidates, err := store.BackfillCandidates(ctx, 500)
	require.NoError(t, err)
	require.Contains(t, candidates, articleID.String())

	_, err = pool.Exec(ctx, `
		UPDATE source_collection_profiles SET article_method = 'html_static'
		WHERE endpoint_id = $1 AND active
	`, endpointID)
	require.NoError(t, err)
	candidates, err = store.BackfillCandidates(ctx, 500)
	require.NoError(t, err)
	require.Contains(t, candidates, articleID.String())
	report, err := store.BackfillReport(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, report.Eligible, 1)
	require.GreaterOrEqual(t, report.Queueable, 1)
}
