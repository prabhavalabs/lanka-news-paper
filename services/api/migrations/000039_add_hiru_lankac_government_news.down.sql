BEGIN;

CREATE TEMP TABLE migration_000039_source_ids ON COMMIT DROP AS
SELECT id
FROM sources
WHERE website IN (
  'https://hirunews.lk',
  'https://www.lankacnews.com',
  'https://sinhala.news.lk'
);

CREATE TEMP TABLE migration_000039_endpoint_ids ON COMMIT DROP AS
SELECT endpoint.id
FROM source_endpoints endpoint
WHERE endpoint.source_id IN (SELECT id FROM migration_000039_source_ids)
  AND endpoint.url IN (
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

UPDATE articles
SET public_status = 'held'
WHERE source_id IN (SELECT id FROM migration_000039_source_ids);

UPDATE source_collection_profiles
SET active = false
WHERE created_by = 'migration-000039'
  AND endpoint_id IN (SELECT id FROM migration_000039_endpoint_ids);

WITH previous AS (
  SELECT DISTINCT ON (profile.endpoint_id) profile.id
  FROM source_collection_profiles profile
  WHERE profile.endpoint_id IN (SELECT id FROM migration_000039_endpoint_ids)
    AND profile.created_by <> 'migration-000039'
  ORDER BY profile.endpoint_id, profile.version DESC
)
UPDATE source_collection_profiles profile
SET active = true
FROM previous
WHERE profile.id = previous.id;

UPDATE source_compliance_reviews
SET active = false
WHERE reviewed_by = 'operator:user-approved-000039'
  AND source_id IN (SELECT id FROM migration_000039_source_ids);

WITH previous AS (
  SELECT DISTINCT ON (review.source_id) review.id
  FROM source_compliance_reviews review
  WHERE review.source_id IN (SELECT id FROM migration_000039_source_ids)
    AND review.reviewed_by <> 'operator:user-approved-000039'
  ORDER BY review.source_id, review.version DESC
)
UPDATE source_compliance_reviews review
SET active = true
FROM previous
WHERE review.id = previous.id;

UPDATE source_endpoints
SET paused = true,
    health_state = 'unknown'
WHERE id IN (SELECT id FROM migration_000039_endpoint_ids);

UPDATE source_endpoints endpoint
SET url = 'https://v3sin.news.lk/fetures/itemlist/category/6-news?format=feed&type=rss',
    health_state = 'stale',
    last_error = 'Legacy official feed has not published since 2024-12-19'
FROM sources source
WHERE endpoint.source_id = source.id
  AND source.website = 'https://sinhala.news.lk'
  AND endpoint.url = 'https://sinhala.news.lk/current-affairs?format=feed&type=rss';

UPDATE sources
SET active = false,
    archived_at = clock_timestamp(),
    description = CASE website
      WHEN 'https://hirunews.lk'
        THEN 'Registered publisher; held until an official ingestible Sinhala feed is available.'
      WHEN 'https://sinhala.news.lk'
        THEN 'Registered publisher; held because its legacy official feed is stale.'
      ELSE description
    END
WHERE id IN (SELECT id FROM migration_000039_source_ids);

COMMIT;
