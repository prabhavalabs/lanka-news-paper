package content

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/temoto/robotstxt"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/registry"
)

type Store struct {
	pool    *pgxpool.Pool
	fetcher *safeFetcher
}

type RateLimitError struct {
	Host       string
	RetryAfter time.Duration
}

type BackfillReport struct {
	TotalArticles     int `json:"total_articles"`
	Stored            int `json:"stored"`
	AwaitingApproval  int `json:"awaiting_approval"`
	NeedsStaticRecipe int `json:"needs_static_recipe"`
	NeedsRobotsReview int `json:"needs_robots_review"`
	Eligible          int `json:"eligible"`
	Queued            int `json:"queued"`
	Queueable         int `json:"queueable"`
	RetryLimitBlocked int `json:"retry_limit_blocked"`
}

func (err *RateLimitError) Error() string {
	return fmt.Sprintf("crawler rate limit is active for %s", err.Host)
}

type eligibility struct {
	articleID             string
	sourceID              string
	originalURL           string
	profileID             string
	articleMethod         string
	config                registry.CollectionConfig
	minDelaySeconds       int
	requestTimeoutSeconds int
	complianceID          string
	complianceStatus      string
	robotsURL             string
	robotsCheckedAt       *time.Time
	robotsAllowed         *bool
	allowFullTextStorage  bool
	retentionDays         int
}

func (policy eligibility) storageApproved() bool {
	return policy.allowFullTextStorage && (policy.complianceStatus == "approved" || policy.complianceStatus == "restricted")
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, fetcher: newSafeFetcher()}
}

// CaptureStructured stores a body already supplied by an RSS/Atom/API payload.
// It is a no-op unless an active collection profile and compliance review both
// authorize this exact acquisition method.
func (store *Store) CaptureStructured(ctx context.Context, articleID, rawBody, method string) (bool, error) {
	if method != "feed_content" && method != "api_content" {
		return false, fmt.Errorf("unsupported structured-content method")
	}
	policy, err := store.eligibility(ctx, articleID)
	if err != nil {
		return false, err
	}
	if !policy.storageApproved() || policy.articleMethod != method || strings.TrimSpace(rawBody) == "" {
		return false, nil
	}
	body, err := extractStructuredText(rawBody)
	if err != nil {
		return false, err
	}
	result := Extraction{
		BodyText:     body,
		Method:       method,
		Characters:   len([]rune(body)),
		SinhalaRatio: sinhalaRatio(body),
	}
	if err := validateExtraction(result, policy.config); err != nil {
		return false, err
	}
	return store.save(ctx, policy, result, method, policy.originalURL)
}

func (store *Store) NeedsStaticFetch(ctx context.Context, articleID string) (bool, error) {
	policy, err := store.eligibility(ctx, articleID)
	if err != nil {
		return false, err
	}
	if !policy.storageApproved() || policy.articleMethod != "html_static" {
		return false, nil
	}
	var exists bool
	if err := store.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM article_contents WHERE article_id = $1 AND current)
	`, articleID).Scan(&exists); err != nil {
		return false, err
	}
	return !exists, nil
}

func (store *Store) Enrich(ctx context.Context, articleID string) error {
	policy, err := store.eligibility(ctx, articleID)
	if err != nil {
		return err
	}
	if !policy.storageApproved() || policy.articleMethod != "html_static" {
		return nil
	}
	if policy.complianceStatus != "approved" && policy.complianceStatus != "restricted" {
		return fmt.Errorf("static crawling is not approved by the active compliance review")
	}
	if policy.robotsCheckedAt == nil || policy.robotsAllowed == nil || !*policy.robotsAllowed {
		return fmt.Errorf("static crawling requires a reviewed and allowed robots policy")
	}
	parsedArticleURL, err := validateOutboundURL(policy.originalURL, policy.config.AllowedHosts)
	if err != nil {
		return store.recordBlocked(ctx, policy, err)
	}
	if !urlMatchesPatterns(parsedArticleURL.String(), policy.config.ArticleURLPatterns) {
		return store.recordBlocked(ctx, policy, fmt.Errorf("article URL does not match the configured patterns"))
	}
	attemptID, err := store.startAttempt(ctx, policy)
	if err != nil {
		return err
	}
	failed := func(status string, result fetchResult, cause error) error {
		store.finishAttempt(ctx, attemptID, status, result, 0, cause)
		return cause
	}
	timeout := time.Duration(policy.requestTimeoutSeconds) * time.Second
	userAgent := policy.config.UserAgent
	if userAgent == "" {
		userAgent = "SNAPBot/1.0"
	}
	robotsURL := policy.robotsURL
	if robotsURL == "" {
		robotsURL = canonicalRobotsURL(policy.originalURL)
	}

	robotsResult, robotsErr := store.loadRobotsPolicy(ctx, policy, robotsURL, timeout, userAgent)
	if robotsErr != nil && robotsResult.StatusCode != 404 {
		return failed("blocked", robotsResult, fmt.Errorf("robots check failed: %w", robotsErr))
	}
	if robotsResult.StatusCode != 404 {
		robots, parseErr := robotstxt.FromBytes(robotsResult.Body)
		if parseErr != nil {
			return failed("blocked", robotsResult, fmt.Errorf("parse robots policy: %w", parseErr))
		}
		group := robots.FindGroup(robotsUserAgent(userAgent))
		path := parsedArticleURL.EscapedPath()
		if path == "" {
			path = "/"
		}
		if !group.Test(path) {
			return failed("blocked", robotsResult, fmt.Errorf("robots policy disallows this article path"))
		}
		if group.CrawlDelay > 0 {
			policy.minDelaySeconds = effectiveCrawlDelaySeconds(policy.minDelaySeconds, group.CrawlDelay)
		}
	}
	if err := store.acquireDomainLease(ctx, parsedArticleURL.Hostname(), policy.minDelaySeconds); err != nil {
		return failed("skipped", fetchResult{FinalURL: parsedArticleURL.String()}, err)
	}

	result, err := store.fetcher.fetchHTML(ctx, parsedArticleURL.String(), policy.config.AllowedHosts, timeout, userAgent)
	if err != nil {
		return failed("failed", result, err)
	}
	extraction, err := extractStaticHTML(result.Body, policy.config)
	if err != nil {
		return failed("failed", result, err)
	}
	if err := validateExtraction(extraction, policy.config); err != nil {
		return failed("failed", result, err)
	}
	if _, err := store.save(ctx, policy, extraction, "html_static", result.FinalURL); err != nil {
		return failed("failed", result, err)
	}
	if extraction.Author != "" {
		_, _ = store.pool.Exec(ctx, `
			UPDATE articles SET author = CASE WHEN COALESCE(author, '') = '' THEN $2 ELSE author END
			WHERE id = $1
		`, articleID, extraction.Author)
	}
	store.finishAttempt(ctx, attemptID, "succeeded", result, extraction.Characters, nil)
	return nil
}

func (store *Store) eligibility(ctx context.Context, articleID string) (eligibility, error) {
	var policy eligibility
	var rawConfig []byte
	err := store.pool.QueryRow(ctx, `
		SELECT article.id::text, article.source_id::text, article.original_url,
		       profile.id::text, profile.article_method, profile.config,
		       profile.min_delay_seconds, profile.request_timeout_seconds,
		       compliance.id::text, compliance.status, COALESCE(compliance.robots_url, ''),
		       compliance.robots_checked_at, compliance.robots_allowed,
		       compliance.allow_full_text_storage, rights.raw_payload_retention_days
		FROM articles article
		JOIN rights_profiles rights ON rights.id = article.rights_profile_id
		JOIN source_collection_profiles profile
		  ON profile.endpoint_id = article.endpoint_id AND profile.active
		JOIN source_compliance_reviews compliance
		  ON compliance.source_id = article.source_id AND compliance.active
		WHERE article.id = $1
	`, articleID).Scan(
		&policy.articleID, &policy.sourceID, &policy.originalURL,
		&policy.profileID, &policy.articleMethod, &rawConfig,
		&policy.minDelaySeconds, &policy.requestTimeoutSeconds,
		&policy.complianceID, &policy.complianceStatus, &policy.robotsURL,
		&policy.robotsCheckedAt, &policy.robotsAllowed,
		&policy.allowFullTextStorage, &policy.retentionDays,
	)
	if err != nil {
		return eligibility{}, fmt.Errorf("load article collection policy: %w", err)
	}
	if err := json.Unmarshal(rawConfig, &policy.config); err != nil {
		return eligibility{}, fmt.Errorf("decode article collection policy: %w", err)
	}
	return policy, nil
}

func (store *Store) save(ctx context.Context, policy eligibility, extraction Extraction, method, sourceURL string) (bool, error) {
	hash := sha256.Sum256([]byte(extraction.BodyText))
	contentHash := hex.EncodeToString(hash[:])
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, policy.articleID); err != nil {
		return false, err
	}
	var existingHash string
	err = tx.QueryRow(ctx, `
		SELECT content_hash FROM article_contents WHERE article_id = $1 AND current
	`, policy.articleID).Scan(&existingHash)
	if err == nil && existingHash == contentHash {
		return false, tx.Commit(ctx)
	}
	if err != nil && err != pgx.ErrNoRows {
		return false, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE article_contents SET current = false WHERE article_id = $1 AND current
	`, policy.articleID); err != nil {
		return false, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO article_contents (
			article_id, version, current, body_text, acquisition_method,
			source_url, content_hash, extractor_version, collection_profile_id,
			compliance_review_id, retention_until
		)
		SELECT $1, COALESCE(max(version), 0) + 1, true, $2, $3, $4, $5, $6, $7, $8,
		       CASE WHEN $9 > 0 THEN clock_timestamp() + make_interval(days => $9) ELSE NULL END
		FROM article_contents
		WHERE article_id = $1
	`, policy.articleID, extraction.BodyText, method, sourceURL, contentHash,
		extractorVersion, policy.profileID, policy.complianceID, policy.retentionDays)
	if err != nil {
		return false, fmt.Errorf("store article content: %w", err)
	}
	return true, tx.Commit(ctx)
}

func (store *Store) acquireDomainLease(ctx context.Context, host string, minDelaySeconds int) error {
	host = normalizeHost(host)
	var acquired time.Time
	err := store.pool.QueryRow(ctx, `
		INSERT INTO crawl_domain_leases (host, last_request_at)
		VALUES ($1, clock_timestamp())
		ON CONFLICT (host) DO UPDATE
		SET last_request_at = EXCLUDED.last_request_at
		WHERE crawl_domain_leases.last_request_at
		      <= clock_timestamp() - make_interval(secs => $2)
		RETURNING last_request_at
	`, host, minDelaySeconds).Scan(&acquired)
	if err == pgx.ErrNoRows {
		return &RateLimitError{Host: host, RetryAfter: time.Duration(minDelaySeconds) * time.Second}
	}
	return err
}

func effectiveCrawlDelaySeconds(configured int, robotsDelay time.Duration) int {
	robotsSeconds := int((robotsDelay + time.Second - 1) / time.Second)
	if robotsSeconds > configured {
		return robotsSeconds
	}
	return configured
}

func (store *Store) loadRobotsPolicy(
	ctx context.Context,
	policy eligibility,
	robotsURL string,
	timeout time.Duration,
	userAgent string,
) (fetchResult, error) {
	var cachedBody string
	var cachedStatus int
	err := store.pool.QueryRow(ctx, `
		SELECT body_text, http_status
		FROM source_robots_cache
		WHERE source_id = $1 AND compliance_review_id = $2
		  AND robots_url = $3 AND expires_at > clock_timestamp()
	`, policy.sourceID, policy.complianceID, robotsURL).Scan(&cachedBody, &cachedStatus)
	if err == nil {
		return fetchResult{
			Body:        []byte(cachedBody),
			FinalURL:    robotsURL,
			StatusCode:  cachedStatus,
			ContentType: "text/plain",
		}, nil
	}
	if err != pgx.ErrNoRows {
		return fetchResult{}, fmt.Errorf("load robots policy cache: %w", err)
	}
	parsed, err := validateOutboundURL(robotsURL, policy.config.AllowedHosts)
	if err != nil {
		return fetchResult{}, err
	}
	if err := store.acquireDomainLease(ctx, parsed.Hostname(), policy.minDelaySeconds); err != nil {
		return fetchResult{FinalURL: robotsURL}, err
	}
	result, fetchErr := store.fetcher.fetchRobots(ctx, robotsURL, policy.config.AllowedHosts, timeout, userAgent)
	if fetchErr != nil && result.StatusCode != http.StatusNotFound {
		return result, fetchErr
	}
	body := string(result.Body)
	if result.StatusCode == http.StatusNotFound {
		body = ""
		fetchErr = nil
	}
	_, err = store.pool.Exec(ctx, `
		INSERT INTO source_robots_cache (
			source_id, compliance_review_id, robots_url, body_text,
			http_status, fetched_at, expires_at
		) VALUES ($1, $2, $3, $4, $5, clock_timestamp(), clock_timestamp() + interval '24 hours')
		ON CONFLICT (source_id) DO UPDATE SET
			compliance_review_id = EXCLUDED.compliance_review_id,
			robots_url = EXCLUDED.robots_url,
			body_text = EXCLUDED.body_text,
			http_status = EXCLUDED.http_status,
			fetched_at = EXCLUDED.fetched_at,
			expires_at = EXCLUDED.expires_at
	`, policy.sourceID, policy.complianceID, robotsURL, body, result.StatusCode)
	if err != nil {
		return result, fmt.Errorf("cache robots policy: %w", err)
	}
	return result, fetchErr
}

func (store *Store) startAttempt(ctx context.Context, policy eligibility) (string, error) {
	var attemptID string
	err := store.pool.QueryRow(ctx, `
		INSERT INTO crawl_attempts (article_id, source_id, collection_profile_id, requested_url)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text
	`, policy.articleID, policy.sourceID, policy.profileID, policy.originalURL).Scan(&attemptID)
	return attemptID, err
}

func (store *Store) finishAttempt(ctx context.Context, attemptID, status string, result fetchResult, characters int, cause error) {
	detail := ""
	if cause != nil {
		detail = cause.Error()
		if len(detail) > 2000 {
			detail = detail[:2000]
		}
	}
	_, _ = store.pool.Exec(ctx, `
		UPDATE crawl_attempts
		SET final_url = NULLIF($2, ''), status = $3, http_status = NULLIF($4, 0),
		    response_bytes = $5, duration_ms = $6, extractor = $7,
		    extracted_characters = $8, error_detail = NULLIF($9, ''),
		    finished_at = clock_timestamp()
		WHERE id = $1
	`, attemptID, result.FinalURL, status, result.StatusCode, len(result.Body),
		int(result.Duration.Milliseconds()), extractorVersion, characters, detail)
}

func (store *Store) recordBlocked(ctx context.Context, policy eligibility, cause error) error {
	attemptID, err := store.startAttempt(ctx, policy)
	if err == nil {
		store.finishAttempt(ctx, attemptID, "blocked", fetchResult{}, 0, cause)
	}
	return cause
}

func urlMatchesPatterns(value string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pattern := range patterns {
		compiled, err := regexp.Compile(pattern)
		if err == nil && compiled.MatchString(value) {
			return true
		}
	}
	return false
}

func robotsUserAgent(value string) string {
	value = strings.TrimSpace(value)
	if separator := strings.IndexAny(value, "/ "); separator >= 0 {
		value = value[:separator]
	}
	if value == "" {
		return "SNAPBot"
	}
	return value
}

func canonicalRobotsURL(articleURL string) string {
	parsed, err := url.Parse(articleURL)
	if err != nil {
		return ""
	}
	return "https://" + parsed.Host + "/robots.txt"
}

// DeleteExpired removes one bounded batch of restricted full-text bodies after
// their configured rights retention window. Article metadata remains.
func (store *Store) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := store.pool.Exec(ctx, `
		WITH expired AS (
			SELECT id
			FROM article_contents
			WHERE retention_until IS NOT NULL AND retention_until <= clock_timestamp()
			ORDER BY retention_until, id
			LIMIT 1000
			FOR UPDATE SKIP LOCKED
		)
		DELETE FROM article_contents content
		USING expired
		WHERE content.id = expired.id
	`)
	if err != nil {
		return 0, fmt.Errorf("delete expired article content: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (store *Store) BackfillCandidates(ctx context.Context, limit int) ([]string, error) {
	if limit < 1 || limit > 500 {
		return nil, fmt.Errorf("backfill limit must be between 1 and 500")
	}
	rows, err := store.pool.Query(ctx, `
		SELECT article.id::text
		FROM articles article
		JOIN source_collection_profiles profile
		  ON profile.endpoint_id = article.endpoint_id AND profile.active
		JOIN source_compliance_reviews compliance
		  ON compliance.source_id = article.source_id AND compliance.active
		WHERE profile.article_method = 'html_static'
		  AND compliance.status IN ('approved', 'restricted')
		  AND compliance.allow_full_text_storage
		  AND compliance.robots_checked_at IS NOT NULL
		  AND compliance.robots_allowed IS TRUE
		  AND NOT EXISTS (
			SELECT 1 FROM article_contents body
			WHERE body.article_id = article.id AND body.current
		  )
		  AND (
			SELECT count(*) FROM crawl_attempts attempt
			WHERE attempt.article_id = article.id
			  AND attempt.collection_profile_id = profile.id
			  AND attempt.status IN ('failed', 'blocked')
		  ) < 3
		  AND NOT EXISTS (
			SELECT 1 FROM river_job job
			WHERE job.kind = 'article.content'
			  AND job.args->>'article_id' = article.id::text
			  AND job.state IN ('available', 'pending', 'retryable', 'running', 'scheduled')
		  )
		ORDER BY article.received_at, article.id
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("select article content backfill: %w", err)
	}
	defer rows.Close()
	articleIDs := make([]string, 0, limit)
	for rows.Next() {
		var articleID string
		if err := rows.Scan(&articleID); err != nil {
			return nil, err
		}
		articleIDs = append(articleIDs, articleID)
	}
	return articleIDs, rows.Err()
}

func (store *Store) BackfillReport(ctx context.Context) (BackfillReport, error) {
	var report BackfillReport
	err := store.pool.QueryRow(ctx, `
		WITH state AS (
			SELECT article.id,
			       body.article_id IS NOT NULL AS stored,
			       COALESCE(
			         compliance.status IN ('approved', 'restricted')
			         AND compliance.allow_full_text_storage,
			         false
			       ) AS approved,
			       COALESCE(profile.article_method, 'metadata_only') AS article_method,
			       COALESCE(
			         compliance.robots_checked_at IS NOT NULL
			         AND compliance.robots_allowed IS TRUE,
			         false
			       ) AS robots_ready,
			       COALESCE(failures.total, 0) AS failures,
			       active_job.article_id IS NOT NULL AS queued
			FROM articles article
			LEFT JOIN source_collection_profiles profile
			  ON profile.endpoint_id = article.endpoint_id AND profile.active
			LEFT JOIN source_compliance_reviews compliance
			  ON compliance.source_id = article.source_id AND compliance.active
			LEFT JOIN LATERAL (
				SELECT body.article_id FROM article_contents body
				WHERE body.article_id = article.id AND body.current
				LIMIT 1
			) body ON true
			LEFT JOIN LATERAL (
				SELECT count(*)::integer AS total
				FROM crawl_attempts attempt
				WHERE attempt.article_id = article.id
				  AND attempt.collection_profile_id = profile.id
				  AND attempt.status IN ('failed', 'blocked')
			) failures ON true
			LEFT JOIN LATERAL (
				SELECT job.args->>'article_id' AS article_id
				FROM river_job job
				WHERE job.kind = 'article.content'
				  AND job.args->>'article_id' = article.id::text
				  AND job.state IN ('available', 'pending', 'retryable', 'running', 'scheduled')
				LIMIT 1
			) active_job ON true
		)
		SELECT count(*)::integer,
		       count(*) FILTER (WHERE stored)::integer,
		       count(*) FILTER (WHERE NOT stored AND NOT approved)::integer,
		       count(*) FILTER (WHERE NOT stored AND approved AND article_method <> 'html_static')::integer,
		       count(*) FILTER (WHERE NOT stored AND approved AND article_method = 'html_static' AND NOT robots_ready)::integer,
		       count(*) FILTER (WHERE NOT stored AND approved AND article_method = 'html_static' AND robots_ready AND failures < 3)::integer,
		       count(*) FILTER (WHERE queued)::integer,
		       count(*) FILTER (WHERE NOT stored AND approved AND article_method = 'html_static' AND robots_ready AND failures < 3 AND NOT queued)::integer,
		       count(*) FILTER (WHERE NOT stored AND approved AND article_method = 'html_static' AND robots_ready AND failures >= 3)::integer
		FROM state
	`).Scan(
		&report.TotalArticles, &report.Stored, &report.AwaitingApproval,
		&report.NeedsStaticRecipe, &report.NeedsRobotsReview, &report.Eligible, &report.Queued,
		&report.Queueable, &report.RetryLimitBlocked,
	)
	if err != nil {
		return BackfillReport{}, fmt.Errorf("report article content backfill: %w", err)
	}
	return report, nil
}
