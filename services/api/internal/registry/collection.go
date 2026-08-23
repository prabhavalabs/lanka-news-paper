package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	maxCollectionListItems = 20
	maxSelectorLength      = 500
)

// CollectionConfig is deliberately declarative. Administrators may configure
// selectors and RE2-compatible URL patterns, but never executable crawler code.
type CollectionConfig struct {
	DiscoveryURLs        []string `json:"discovery_urls"`
	AllowedHosts         []string `json:"allowed_hosts"`
	ArticleURLPatterns   []string `json:"article_url_patterns"`
	LinkSelector         string   `json:"link_selector"`
	TitleSelector        string   `json:"title_selector"`
	PublishedSelector    string   `json:"published_selector"`
	AuthorSelector       string   `json:"author_selector"`
	ContentSelector      string   `json:"content_selector"`
	ExcludeSelectors     []string `json:"exclude_selectors"`
	PaginationMode       string   `json:"pagination_mode"`
	NextPageSelector     string   `json:"next_page_selector"`
	PageParameter        string   `json:"page_parameter"`
	UserAgent            string   `json:"user_agent"`
	MinContentCharacters int      `json:"min_content_characters"`
	MinimumSinhalaRatio  float64  `json:"minimum_sinhala_ratio"`
}

type CollectionProfile struct {
	ID                    string           `json:"id"`
	SourceID              string           `json:"source_id"`
	EndpointID            string           `json:"endpoint_id"`
	Version               int              `json:"version"`
	DiscoveryMethod       string           `json:"discovery_method"`
	ArticleMethod         string           `json:"article_method"`
	Config                CollectionConfig `json:"config"`
	MinDelaySeconds       int              `json:"min_delay_seconds"`
	MaxRequestsPerRun     int              `json:"max_requests_per_run"`
	MaxPages              int              `json:"max_pages"`
	RequestTimeoutSeconds int              `json:"request_timeout_seconds"`
	CreatedBy             string           `json:"created_by"`
	ActivatedAt           *time.Time       `json:"activated_at"`
	CreatedAt             time.Time        `json:"created_at"`
}

type ComplianceReview struct {
	ID                   string     `json:"id"`
	SourceID             string     `json:"source_id"`
	Version              int        `json:"version"`
	Status               string     `json:"status"`
	RobotsURL            string     `json:"robots_url"`
	RobotsCheckedAt      *time.Time `json:"robots_checked_at"`
	RobotsAllowed        *bool      `json:"robots_allowed"`
	TermsURLs            []string   `json:"terms_urls"`
	AllowDiscovery       bool       `json:"allow_discovery"`
	AllowFullTextStorage bool       `json:"allow_full_text_storage"`
	AllowAIProcessing    bool       `json:"allow_ai_processing"`
	AllowEmbeddings      bool       `json:"allow_embeddings"`
	AllowTraining        bool       `json:"allow_training"`
	AllowPublicFullText  bool       `json:"allow_public_full_text"`
	Notes                string     `json:"notes"`
	ReviewedBy           string     `json:"reviewed_by"`
	ReviewedAt           *time.Time `json:"reviewed_at"`
	ReviewOn             *time.Time `json:"review_on"`
	CreatedAt            time.Time  `json:"created_at"`
}

func (store *Store) ListCollectionProfiles(ctx context.Context, sourceID string) ([]CollectionProfile, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT profile.id::text, profile.source_id::text, profile.endpoint_id::text,
		       profile.version, profile.discovery_method, profile.article_method,
		       profile.config, profile.min_delay_seconds, profile.max_requests_per_run,
		       profile.max_pages, profile.request_timeout_seconds, profile.created_by,
		       profile.activated_at, profile.created_at
		FROM source_collection_profiles profile
		WHERE profile.source_id = $1 AND profile.active
		ORDER BY profile.created_at, profile.endpoint_id
	`, sourceID)
	if err != nil {
		return nil, fmt.Errorf("list collection profiles: %w", err)
	}
	defer rows.Close()

	profiles := make([]CollectionProfile, 0)
	for rows.Next() {
		profile, err := scanCollectionProfile(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, rows.Err()
}

func (store *Store) SaveCollectionProfile(ctx context.Context, sourceID, actor string, profile CollectionProfile) (CollectionProfile, error) {
	var endpointURL, website, endpointType string
	if err := store.pool.QueryRow(ctx, `
		SELECT endpoint.url, COALESCE(source.website, ''), endpoint.endpoint_type
		FROM source_endpoints endpoint
		JOIN sources source ON source.id = endpoint.source_id
		WHERE endpoint.id = $1 AND endpoint.source_id = $2 AND source.archived_at IS NULL
	`, profile.EndpointID, sourceID).Scan(&endpointURL, &website, &endpointType); err != nil {
		return CollectionProfile{}, fmt.Errorf("find collection endpoint: %w", err)
	}
	if profile.DiscoveryMethod == "" {
		profile.DiscoveryMethod = endpointType
	}
	if err := validateCollectionProfile(&profile, endpointURL, website); err != nil {
		return CollectionProfile{}, err
	}
	encodedConfig, err := json.Marshal(profile.Config)
	if err != nil {
		return CollectionProfile{}, fmt.Errorf("encode collection config: %w", err)
	}

	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return CollectionProfile{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "collection:"+profile.EndpointID); err != nil {
		return CollectionProfile{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE source_collection_profiles SET active = false
		WHERE endpoint_id = $1 AND active
	`, profile.EndpointID); err != nil {
		return CollectionProfile{}, err
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO source_collection_profiles (
			source_id, endpoint_id, version, active, discovery_method, article_method,
			config, min_delay_seconds, max_requests_per_run, max_pages,
			request_timeout_seconds, created_by, activated_at
		)
		SELECT $1, $2, COALESCE(max(version), 0) + 1, true, $3, $4,
		       $5::jsonb, $6, $7, $8, $9, $10, clock_timestamp()
		FROM source_collection_profiles
		WHERE endpoint_id = $2
		RETURNING id::text, version, activated_at, created_at
	`, sourceID, profile.EndpointID, profile.DiscoveryMethod, profile.ArticleMethod,
		string(encodedConfig), profile.MinDelaySeconds, profile.MaxRequestsPerRun,
		profile.MaxPages, profile.RequestTimeoutSeconds, actor,
	).Scan(&profile.ID, &profile.Version, &profile.ActivatedAt, &profile.CreatedAt)
	if err != nil {
		return CollectionProfile{}, fmt.Errorf("save collection profile: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return CollectionProfile{}, err
	}
	profile.SourceID = sourceID
	profile.CreatedBy = actor
	return profile, nil
}

func (store *Store) GetComplianceReview(ctx context.Context, sourceID string) (ComplianceReview, error) {
	row := store.pool.QueryRow(ctx, `
		SELECT id::text, source_id::text, version, status, COALESCE(robots_url, ''),
		       robots_checked_at, robots_allowed, terms_urls, allow_discovery,
		       allow_full_text_storage, allow_ai_processing, allow_embeddings,
		       allow_training, allow_public_full_text, notes, reviewed_by,
		       reviewed_at, review_on::timestamptz, created_at
		FROM source_compliance_reviews
		WHERE source_id = $1 AND active
	`, sourceID)
	return scanComplianceReview(row)
}

func (store *Store) SaveComplianceReview(ctx context.Context, sourceID, actor string, review ComplianceReview) (ComplianceReview, error) {
	var website string
	if err := store.pool.QueryRow(ctx, `
		SELECT COALESCE(website, '') FROM sources
		WHERE id = $1 AND archived_at IS NULL
	`, sourceID).Scan(&website); err != nil {
		return ComplianceReview{}, fmt.Errorf("find compliance source: %w", err)
	}
	if err := validateComplianceReview(&review, website); err != nil {
		return ComplianceReview{}, err
	}

	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return ComplianceReview{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "compliance:"+sourceID); err != nil {
		return ComplianceReview{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE source_compliance_reviews SET active = false
		WHERE source_id = $1 AND active
	`, sourceID); err != nil {
		return ComplianceReview{}, err
	}
	if review.Status == "approved" || review.Status == "restricted" || review.Status == "denied" {
		now := time.Now().UTC()
		review.ReviewedAt = &now
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO source_compliance_reviews (
			source_id, version, active, status, robots_url, robots_checked_at,
			robots_allowed, terms_urls, allow_discovery, allow_full_text_storage,
			allow_ai_processing, allow_embeddings, allow_training,
			allow_public_full_text, notes, reviewed_by, reviewed_at, review_on
		)
		SELECT $1, COALESCE(max(version), 0) + 1, true, $2, NULLIF($3, ''), $4,
		       $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		FROM source_compliance_reviews
		WHERE source_id = $1
		RETURNING id::text, version, created_at
	`, sourceID, review.Status, review.RobotsURL, review.RobotsCheckedAt,
		review.RobotsAllowed, review.TermsURLs, review.AllowDiscovery,
		review.AllowFullTextStorage, review.AllowAIProcessing, review.AllowEmbeddings,
		review.AllowTraining, review.AllowPublicFullText, review.Notes, actor,
		review.ReviewedAt, review.ReviewOn,
	).Scan(&review.ID, &review.Version, &review.CreatedAt)
	if err != nil {
		return ComplianceReview{}, fmt.Errorf("save compliance review: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ComplianceReview{}, err
	}
	review.SourceID = sourceID
	review.ReviewedBy = actor
	return review, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCollectionProfile(row rowScanner) (CollectionProfile, error) {
	var profile CollectionProfile
	var rawConfig []byte
	err := row.Scan(
		&profile.ID, &profile.SourceID, &profile.EndpointID, &profile.Version,
		&profile.DiscoveryMethod, &profile.ArticleMethod, &rawConfig,
		&profile.MinDelaySeconds, &profile.MaxRequestsPerRun, &profile.MaxPages,
		&profile.RequestTimeoutSeconds, &profile.CreatedBy, &profile.ActivatedAt,
		&profile.CreatedAt,
	)
	if err != nil {
		return CollectionProfile{}, err
	}
	if err := json.Unmarshal(rawConfig, &profile.Config); err != nil {
		return CollectionProfile{}, fmt.Errorf("decode collection config: %w", err)
	}
	return profile, nil
}

func scanComplianceReview(row rowScanner) (ComplianceReview, error) {
	var review ComplianceReview
	err := row.Scan(
		&review.ID, &review.SourceID, &review.Version, &review.Status,
		&review.RobotsURL, &review.RobotsCheckedAt, &review.RobotsAllowed,
		&review.TermsURLs, &review.AllowDiscovery, &review.AllowFullTextStorage,
		&review.AllowAIProcessing, &review.AllowEmbeddings, &review.AllowTraining,
		&review.AllowPublicFullText, &review.Notes, &review.ReviewedBy,
		&review.ReviewedAt, &review.ReviewOn, &review.CreatedAt,
	)
	if err != nil {
		return ComplianceReview{}, err
	}
	return review, nil
}

func validateCollectionProfile(profile *CollectionProfile, endpointURL, website string) error {
	discoveryMethods := map[string]bool{
		"rss": true, "atom": true, "json_feed": true, "rest_api": true,
		"sitemap": true, "listing_page": true, "webhook": true, "youtube": true,
	}
	articleMethods := map[string]bool{
		"metadata_only": true, "feed_content": true, "api_content": true, "html_static": true,
	}
	if !discoveryMethods[profile.DiscoveryMethod] {
		return fmt.Errorf("unsupported discovery method")
	}
	if !articleMethods[profile.ArticleMethod] {
		return fmt.Errorf("unsupported article method")
	}
	if profile.MinDelaySeconds < 1 || profile.MinDelaySeconds > 86400 ||
		profile.MaxRequestsPerRun < 1 || profile.MaxRequestsPerRun > 500 ||
		profile.MaxPages < 1 || profile.MaxPages > 100 ||
		profile.RequestTimeoutSeconds < 3 || profile.RequestTimeoutSeconds > 60 {
		return fmt.Errorf("collection limits are outside the allowed range")
	}
	if profile.Config.PaginationMode == "" {
		profile.Config.PaginationMode = "none"
	}
	if profile.Config.UserAgent == "" {
		profile.Config.UserAgent = "SNAPBot/1.0"
	}
	if profile.Config.MinContentCharacters == 0 {
		profile.Config.MinContentCharacters = 200
	}
	if len(profile.Config.UserAgent) > 200 || strings.ContainsAny(profile.Config.UserAgent, "\r\n") {
		return fmt.Errorf("invalid crawler user agent")
	}
	if profile.Config.MinContentCharacters < 100 || profile.Config.MinContentCharacters > 50000 {
		return fmt.Errorf("minimum content characters must be between 100 and 50000")
	}
	if profile.Config.MinimumSinhalaRatio < 0 || profile.Config.MinimumSinhalaRatio > 1 {
		return fmt.Errorf("minimum Sinhala ratio must be between 0 and 1")
	}
	if profile.Config.PaginationMode != "none" && profile.Config.PaginationMode != "next_link" && profile.Config.PaginationMode != "page_parameter" {
		return fmt.Errorf("unsupported pagination mode")
	}
	if len(profile.Config.DiscoveryURLs) == 0 {
		profile.Config.DiscoveryURLs = []string{endpointURL}
	}
	if err := validateListLengths(profile.Config); err != nil {
		return err
	}

	trustedRoots := compactHosts(endpointURL, website)
	if len(profile.Config.AllowedHosts) == 0 {
		profile.Config.AllowedHosts = append([]string(nil), trustedRoots...)
	}
	allowedHosts := make([]string, 0, len(profile.Config.AllowedHosts))
	for _, value := range profile.Config.AllowedHosts {
		host := cleanHost(value)
		if host == "" || !hostMatchesAny(host, trustedRoots) {
			return fmt.Errorf("allowed host %q is outside this source", value)
		}
		allowedHosts = append(allowedHosts, host)
	}
	profile.Config.AllowedHosts = uniqueStrings(allowedHosts)
	for _, rawURL := range profile.Config.DiscoveryURLs {
		parsed, err := validateHTTPSURL(rawURL)
		if err != nil || !hostMatchesAny(cleanHost(parsed.Hostname()), profile.Config.AllowedHosts) {
			return fmt.Errorf("discovery URL must be HTTPS and use an allowed source host")
		}
	}
	for _, pattern := range profile.Config.ArticleURLPatterns {
		if len(pattern) > maxSelectorLength {
			return fmt.Errorf("article URL pattern is too long")
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid article URL pattern: %w", err)
		}
	}
	for _, selector := range append([]string{
		profile.Config.LinkSelector, profile.Config.TitleSelector,
		profile.Config.PublishedSelector, profile.Config.AuthorSelector,
		profile.Config.ContentSelector, profile.Config.NextPageSelector,
	}, profile.Config.ExcludeSelectors...) {
		if len(selector) > maxSelectorLength {
			return fmt.Errorf("CSS selector is too long")
		}
	}
	if len(profile.Config.PageParameter) > 80 {
		return fmt.Errorf("page parameter is too long")
	}
	return nil
}

func validateComplianceReview(review *ComplianceReview, website string) error {
	statuses := map[string]bool{"pending": true, "approved": true, "restricted": true, "denied": true}
	if !statuses[review.Status] {
		return fmt.Errorf("unsupported compliance status")
	}
	if len(review.Notes) > 10000 {
		return fmt.Errorf("compliance notes are too long")
	}
	if len(review.TermsURLs) > maxCollectionListItems {
		return fmt.Errorf("too many policy URLs")
	}
	if review.RobotsURL != "" {
		if _, err := validateHTTPSURL(review.RobotsURL); err != nil {
			return fmt.Errorf("robots URL must be a valid HTTPS URL")
		}
	}
	for _, value := range review.TermsURLs {
		if _, err := validateHTTPSURL(value); err != nil {
			return fmt.Errorf("policy URLs must use HTTPS")
		}
	}
	if review.Status == "denied" && (review.AllowDiscovery || review.AllowFullTextStorage || review.AllowAIProcessing || review.AllowEmbeddings || review.AllowTraining || review.AllowPublicFullText) {
		return fmt.Errorf("a denied source cannot have collection or processing permissions")
	}
	if review.Status == "pending" && (review.AllowDiscovery || review.AllowFullTextStorage || review.AllowAIProcessing || review.AllowEmbeddings || review.AllowTraining || review.AllowPublicFullText) {
		return fmt.Errorf("a pending review cannot activate collection or processing permissions")
	}
	if review.AllowPublicFullText && !review.AllowFullTextStorage {
		return fmt.Errorf("public full text requires full-text storage permission")
	}
	if (review.AllowEmbeddings || review.AllowTraining) && !review.AllowAIProcessing {
		return fmt.Errorf("embeddings and training require AI-processing permission")
	}
	if review.AllowFullTextStorage && review.Status != "approved" && review.Status != "restricted" {
		return fmt.Errorf("full-text storage requires an approved or restricted review")
	}
	if review.RobotsURL == "" && strings.HasPrefix(website, "https://") {
		parsed, _ := url.Parse(website)
		review.RobotsURL = "https://" + parsed.Host + "/robots.txt"
	}
	return nil
}

func validateListLengths(config CollectionConfig) error {
	for label, size := range map[string]int{
		"discovery URLs":       len(config.DiscoveryURLs),
		"allowed hosts":        len(config.AllowedHosts),
		"article URL patterns": len(config.ArticleURLPatterns),
		"exclude selectors":    len(config.ExcludeSelectors),
	} {
		if size > maxCollectionListItems {
			return fmt.Errorf("too many %s", label)
		}
	}
	return nil
}

func validateHTTPSURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, fmt.Errorf("invalid HTTPS URL")
	}
	if parsed.Port() != "" && parsed.Port() != "443" {
		return nil, fmt.Errorf("non-standard HTTPS ports are not allowed")
	}
	return parsed, nil
}

func compactHosts(values ...string) []string {
	hosts := make([]string, 0, len(values))
	for _, value := range values {
		parsed, err := url.Parse(value)
		if err == nil {
			if host := cleanHost(parsed.Hostname()); host != "" {
				hosts = append(hosts, host)
			}
		}
	}
	return uniqueStrings(hosts)
}

func cleanHost(value string) string {
	value = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	return value
}

func hostMatchesAny(host string, allowed []string) bool {
	for _, root := range allowed {
		if host == root || strings.HasSuffix(host, "."+root) {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

var _ rowScanner = pgx.Row(nil)
