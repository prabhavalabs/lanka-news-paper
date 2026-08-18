BEGIN;

CREATE TEMP TABLE curated_endpoint_ids ON COMMIT DROP AS
SELECT id
FROM source_endpoints
WHERE url IN (
  'https://apisinhala.newsfirst.lk/post/sticky',
  'https://www.praja.lk/feed/',
  'https://www.yukthiya.lk/?rest_route=/wp/v2/posts&per_page=20&_fields=id,date,date_gmt,modified,modified_gmt,link,title,excerpt'
);

CREATE TEMP TABLE curated_article_ids ON COMMIT DROP AS
SELECT id FROM articles WHERE endpoint_id IN (SELECT id FROM curated_endpoint_ids);

UPDATE articles AS article
SET public_status = 'held'
FROM sources AS source
WHERE article.source_id = source.id
  AND source.website = 'https://www.itnnews.lk';

DELETE FROM event_articles WHERE article_id IN (SELECT id FROM curated_article_ids);
DELETE FROM article_versions WHERE article_id IN (SELECT id FROM curated_article_ids);
DELETE FROM articles WHERE id IN (SELECT id FROM curated_article_ids);
DELETE FROM quarantine_payloads WHERE endpoint_id IN (SELECT id FROM curated_endpoint_ids);
DELETE FROM ingestion_runs WHERE endpoint_id IN (SELECT id FROM curated_endpoint_ids);

DELETE FROM rights_profiles
WHERE endpoint_id IN (
  SELECT id FROM source_endpoints WHERE url IN (
    'https://apisinhala.newsfirst.lk/post/sticky',
    'https://www.praja.lk/feed/',
    'https://www.yukthiya.lk/?rest_route=/wp/v2/posts&per_page=20&_fields=id,date,date_gmt,modified,modified_gmt,link,title,excerpt'
  )
);

DELETE FROM source_endpoints
WHERE url IN (
  'https://apisinhala.newsfirst.lk/post/sticky',
  'https://www.praja.lk/feed/',
  'https://www.yukthiya.lk/?rest_route=/wp/v2/posts&per_page=20&_fields=id,date,date_gmt,modified,modified_gmt,link,title,excerpt'
);

DELETE FROM sources
WHERE website IN ('https://www.praja.lk', 'https://www.yukthiya.lk');

UPDATE sources
SET active = false,
    description = CASE website
      WHEN 'https://www.itnnews.lk' THEN 'Official publisher source; held because its former feed now redirects to a web page.'
      WHEN 'https://sinhala.newsfirst.lk' THEN 'Registered publisher; held until an official ingestible Sinhala feed is available.'
      ELSE description
    END
WHERE website IN ('https://www.itnnews.lk', 'https://sinhala.newsfirst.lk');

UPDATE source_endpoints AS endpoint
SET endpoint_type = 'rss',
    url = 'https://www.itnnews.lk/feed/',
    verified_official = false,
    paused = true,
    health_state = 'failed',
    last_error = 'Feed URL redirects to a web page instead of an RSS document'
FROM sources AS source
WHERE endpoint.source_id = source.id
  AND source.website = 'https://www.itnnews.lk';

WITH restored (website, active) AS (
  VALUES
    ('https://aruna.lk', false),
    ('https://sinhala.asianmirror.lk', false),
    ('https://www.colomboxnews.com', true),
    ('https://www.dinamina.lk', false),
    ('https://www.gossiplankahotnews.com', false),
    ('https://www.gossiplankanews.com', true),
    ('https://sinhala.news.lk', false),
    ('https://hirunews.lk', false),
    ('https://www.lankacnews.com', true),
    ('https://lankatruth.com/si', false),
    ('https://sinhala.lankapuvath.lk', false),
    ('https://mawbima.lk', false),
    ('https://mawratanews.lk', false),
    ('https://monara.com', false),
    ('https://nethnews.lk', false),
    ('https://sinhala.newswire.lk', false),
    ('https://pmd.gov.lk/si', false),
    ('https://rata.lk', true),
    ('https://sarasaviya.lk', false),
    ('https://www.silumina.lk', false),
    ('https://supirigossip.com', false)
)
UPDATE sources AS source
SET active = restored.active,
    archived_at = NULL
FROM restored
WHERE source.website = restored.website;

UPDATE source_endpoints AS endpoint
SET paused = false
FROM sources AS source
WHERE endpoint.source_id = source.id
  AND source.website IN (
    'https://www.colomboxnews.com',
    'https://www.gossiplankanews.com',
    'https://www.lankacnews.com',
    'https://rata.lk'
  );

UPDATE articles AS article
SET public_status = 'published'
FROM sources AS source
WHERE article.source_id = source.id
  AND source.website IN (
    'https://www.colomboxnews.com',
    'https://www.gossiplankanews.com',
    'https://www.lankacnews.com',
    'https://rata.lk'
  );

COMMIT;
