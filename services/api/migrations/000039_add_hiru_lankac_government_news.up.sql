BEGIN;

-- These publishers were registered previously but held because their original
-- discovery endpoints were missing or stale. The endpoints below are official,
-- currently parseable, and restricted to private educational analysis.
UPDATE sources
SET active = true,
    archived_at = NULL,
    description = CASE website
      WHEN 'https://hirunews.lk'
        THEN 'Sinhala news service of Asia Broadcasting Corporation (Private) Limited.'
      WHEN 'https://www.lankacnews.com'
        THEN 'Independent Sinhala breaking-news and current-affairs publication.'
      WHEN 'https://sinhala.news.lk'
        THEN 'Official Sinhala news portal of the Department of Government Information.'
      ELSE description
    END
WHERE website IN (
  'https://hirunews.lk',
  'https://www.lankacnews.com',
  'https://sinhala.news.lk'
);

-- Replace News.lk's stale legacy feed with its current official category feed.
UPDATE source_endpoints endpoint
SET endpoint_type = 'rss',
    url = 'https://sinhala.news.lk/current-affairs?format=feed&type=rss'
FROM sources source
WHERE endpoint.source_id = source.id
  AND source.website = 'https://sinhala.news.lk'
  AND endpoint.url = 'https://v3sin.news.lk/fetures/itemlist/category/6-news?format=feed&type=rss';

WITH desired (website, endpoint_type, url) AS (
  VALUES
    ('https://hirunews.lk', 'rest_api', 'https://hirunews.lk/api/fetch_news.php?page=1&category=General'),
    ('https://www.lankacnews.com', 'rss', 'https://www.lankacnews.com/feeds/posts/default?alt=rss'),
    ('https://sinhala.news.lk', 'rss', 'https://sinhala.news.lk/current-affairs?format=feed&type=rss'),
    ('https://sinhala.news.lk', 'rss', 'https://sinhala.news.lk/parliament?format=feed&type=rss'),
    ('https://sinhala.news.lk', 'rss', 'https://sinhala.news.lk/press-release?format=feed&type=rss'),
    ('https://sinhala.news.lk', 'rss', 'https://sinhala.news.lk/cabinet-decisions?format=feed&type=rss'),
    ('https://sinhala.news.lk', 'rss', 'https://sinhala.news.lk/news/economy-development?format=feed&type=rss'),
    ('https://sinhala.news.lk', 'rss', 'https://sinhala.news.lk/news/district-news?format=feed&type=rss'),
    ('https://sinhala.news.lk', 'rss', 'https://sinhala.news.lk/news/features?format=feed&type=rss'),
    ('https://sinhala.news.lk', 'rss', 'https://sinhala.news.lk/news/art-cultural?format=feed&type=rss'),
    ('https://sinhala.news.lk', 'rss', 'https://sinhala.news.lk/news/foreign?format=feed&type=rss'),
    ('https://sinhala.news.lk', 'rss', 'https://sinhala.news.lk/news/sports?format=feed&type=rss')
)
INSERT INTO source_endpoints (
  source_id, endpoint_type, url, polling_interval_seconds,
  verified_official, paused, health_state
)
SELECT source.id,
       desired.endpoint_type,
       desired.url,
       300,
       true,
       false,
       'unknown'
FROM desired
JOIN sources source ON source.website = desired.website
WHERE NOT EXISTS (
  SELECT 1
  FROM source_endpoints endpoint
  WHERE endpoint.source_id = source.id
    AND endpoint.url = desired.url
);

CREATE TEMP TABLE migration_000039_endpoints ON COMMIT DROP AS
SELECT endpoint.id, endpoint.source_id, endpoint.endpoint_type, endpoint.url, source.website
FROM source_endpoints endpoint
JOIN sources source ON source.id = endpoint.source_id
WHERE endpoint.url IN (
  'https://hirunews.lk/api/fetch_news.php?page=1&category=General',
  'https://www.lankacnews.com/feeds/posts/default?alt=rss',
  'https://sinhala.news.lk/current-affairs?format=feed&type=rss',
  'https://sinhala.news.lk/parliament?format=feed&type=rss',
  'https://sinhala.news.lk/press-release?format=feed&type=rss',
  'https://sinhala.news.lk/cabinet-decisions?format=feed&type=rss',
  'https://sinhala.news.lk/news/economy-development?format=feed&type=rss',
  'https://sinhala.news.lk/news/district-news?format=feed&type=rss',
  'https://sinhala.news.lk/news/features?format=feed&type=rss',
  'https://sinhala.news.lk/news/art-cultural?format=feed&type=rss',
  'https://sinhala.news.lk/news/foreign?format=feed&type=rss',
  'https://sinhala.news.lk/news/sports?format=feed&type=rss'
);

UPDATE source_endpoints endpoint
SET polling_interval_seconds = 300,
    verified_official = true,
    paused = false,
    health_state = 'unknown',
    last_error = NULL,
    consecutive_failures = 0,
    backoff_until = NULL,
    etag = NULL,
    last_modified = NULL
WHERE endpoint.id IN (SELECT id FROM migration_000039_endpoints);

-- Keep the raw structured payload out of long-term storage. Extracted full
-- text is private and public APIs continue to expose metadata only.
INSERT INTO rights_profiles (
  source_id, endpoint_id, version, mode, commercial_use,
  allowed_public_fields, excerpt_max_characters, images_allowed,
  logo_allowed, video_embed_allowed, translation_allowed,
  automated_summary_allowed, raw_payload_retention_days, attribution,
  effective_from, review_on, approved_by, approved_at
)
SELECT endpoint.source_id,
       endpoint.id,
       COALESCE(history.max_version, 0) + 1,
       'discovery_only',
       false,
       ARRAY['source_name','headline','published_at','canonical_url','category'],
       0,
       false,
       false,
       false,
       false,
       false,
       0,
       'මූලාශ්‍රය: ' || source.name,
       clock_timestamp(),
       CURRENT_DATE + 90,
       'operator:user-approved-private-research-000039',
       clock_timestamp()
FROM migration_000039_endpoints endpoint
JOIN sources source ON source.id = endpoint.source_id
LEFT JOIN LATERAL (
  SELECT max(rights.version) AS max_version
  FROM rights_profiles rights
  WHERE rights.endpoint_id = endpoint.id
) history ON true;

WITH latest AS (
  SELECT DISTINCT ON (rights.endpoint_id) rights.endpoint_id, rights.id
  FROM rights_profiles rights
  WHERE rights.endpoint_id IN (SELECT id FROM migration_000039_endpoints)
  ORDER BY rights.endpoint_id, rights.version DESC
)
UPDATE articles article
SET rights_profile_id = latest.id
FROM latest
WHERE article.endpoint_id = latest.endpoint_id;

UPDATE source_collection_profiles profile
SET active = false
WHERE profile.endpoint_id IN (SELECT id FROM migration_000039_endpoints)
  AND profile.active;

INSERT INTO source_collection_profiles (
  source_id, endpoint_id, version, active, discovery_method, article_method,
  config, min_delay_seconds, max_requests_per_run, max_pages,
  request_timeout_seconds, created_by, activated_at
)
SELECT endpoint.source_id,
       endpoint.id,
       COALESCE(history.max_version, 0) + 1,
       true,
       endpoint.endpoint_type,
       CASE endpoint.website
         WHEN 'https://hirunews.lk' THEN 'api_content'
         WHEN 'https://www.lankacnews.com' THEN 'feed_content'
         ELSE 'html_static'
       END,
       jsonb_build_object(
         'discovery_urls', jsonb_build_array(endpoint.url),
         'allowed_hosts', CASE endpoint.website
           WHEN 'https://hirunews.lk' THEN jsonb_build_array('hirunews.lk')
           WHEN 'https://www.lankacnews.com' THEN jsonb_build_array('www.lankacnews.com')
           ELSE jsonb_build_array('sinhala.news.lk')
         END,
         'article_url_patterns', CASE endpoint.website
           WHEN 'https://hirunews.lk'
             THEN jsonb_build_array('^https://hirunews[.]lk/[0-9]+/')
           WHEN 'https://www.lankacnews.com'
             THEN jsonb_build_array('^https://www[.]lankacnews[.]com/')
           ELSE jsonb_build_array('^https://sinhala[.]news[.]lk/(current-affairs|parliament|press-release|cabinet-decisions|news)/')
         END,
         'title_selector', CASE WHEN endpoint.website = 'https://sinhala.news.lk'
           THEN 'h1[itemprop="headline"]' ELSE '' END,
         'published_selector', CASE WHEN endpoint.website = 'https://sinhala.news.lk'
           THEN 'time[itemprop="dateCreated"]' ELSE '' END,
         'content_selector', CASE WHEN endpoint.website = 'https://sinhala.news.lk'
           THEN '[itemprop="articleBody"]' ELSE '' END,
         'exclude_selectors', jsonb_build_array(
           'script', 'style', 'nav', 'header', 'footer', 'aside',
           '.advertisement', '.ads', '.related-posts', '.social-share'
         ),
         'pagination_mode', 'none',
         'user_agent', 'SNAPBot/1.0',
         'min_content_characters', CASE WHEN endpoint.website = 'https://sinhala.news.lk' THEN 60 ELSE 100 END,
         'minimum_sinhala_ratio', 0
       ),
       5,
       25,
       1,
       CASE WHEN endpoint.website = 'https://sinhala.news.lk' THEN 30 ELSE 20 END,
       'migration-000039',
       clock_timestamp()
FROM migration_000039_endpoints endpoint
LEFT JOIN LATERAL (
  SELECT max(profile.version) AS max_version
  FROM source_collection_profiles profile
  WHERE profile.endpoint_id = endpoint.id
) history ON true;

UPDATE source_compliance_reviews review
SET active = false
WHERE review.source_id IN (SELECT DISTINCT source_id FROM migration_000039_endpoints)
  AND review.active;

INSERT INTO source_compliance_reviews (
  source_id, version, active, status, robots_url, robots_checked_at,
  robots_allowed, terms_urls, allow_discovery, allow_full_text_storage,
  allow_ai_processing, allow_embeddings, allow_training,
  allow_public_full_text, notes, reviewed_by, reviewed_at, review_on
)
SELECT source.id,
       COALESCE(history.max_version, 0) + 1,
       true,
       'restricted',
       CASE source.website
         WHEN 'https://hirunews.lk' THEN 'https://hirunews.lk/robots.txt'
         WHEN 'https://www.lankacnews.com' THEN 'https://www.lankacnews.com/robots.txt'
         ELSE 'https://sinhala.news.lk/robots.txt'
       END,
       clock_timestamp(),
       true,
       '{}'::text[],
       true,
       true,
       true,
       false,
       false,
       false,
       'Operator-approved private educational analysis using official structured discovery endpoints. Robots rules were reviewed on 2026-08-23. Public full text, embeddings, and model training remain disabled.',
       'operator:user-approved-000039',
       clock_timestamp(),
       CURRENT_DATE + 90
FROM sources source
LEFT JOIN LATERAL (
  SELECT max(review.version) AS max_version
  FROM source_compliance_reviews review
  WHERE review.source_id = source.id
) history ON true
WHERE source.id IN (SELECT DISTINCT source_id FROM migration_000039_endpoints);

UPDATE articles article
SET public_status = 'published'
FROM sources source
WHERE article.source_id = source.id
  AND source.website IN (
    'https://hirunews.lk',
    'https://www.lankacnews.com',
    'https://sinhala.news.lk'
  )
  AND article.public_status = 'held';

COMMIT;
