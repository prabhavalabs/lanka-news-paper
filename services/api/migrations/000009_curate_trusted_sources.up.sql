BEGIN;

-- Keep only accountable publishers with a recent, parseable, official Sinhala endpoint.
CREATE TEMP TABLE rejected_source_ids ON COMMIT DROP AS
SELECT id
FROM sources
WHERE website IN (
  'https://aruna.lk',
  'https://sinhala.asianmirror.lk',
  'https://www.colomboxnews.com',
  'https://www.dinamina.lk',
  'https://www.gossiplankahotnews.com',
  'https://www.gossiplankanews.com',
  'https://sinhala.news.lk',
  'https://hirunews.lk',
  'https://www.lankacnews.com',
  'https://lankatruth.com/si',
  'https://sinhala.lankapuvath.lk',
  'https://mawbima.lk',
  'https://mawratanews.lk',
  'https://monara.com',
  'https://nethnews.lk',
  'https://sinhala.newswire.lk',
  'https://pmd.gov.lk/si',
  'https://rata.lk',
  'https://sarasaviya.lk',
  'https://www.silumina.lk',
  'https://supirigossip.com'
);

UPDATE articles AS article
SET public_status = 'held'
WHERE article.source_id IN (SELECT id FROM rejected_source_ids)
  AND article.public_status = 'published';

UPDATE source_endpoints AS endpoint
SET paused = true
WHERE endpoint.source_id IN (SELECT id FROM rejected_source_ids);

UPDATE sources AS source
SET active = false,
    archived_at = clock_timestamp()
WHERE source.id IN (SELECT id FROM rejected_source_ids)
  AND source.archived_at IS NULL;

UPDATE sources
SET active = true,
    archived_at = NULL,
    description = 'Official Sinhala news service of Independent Television Network Limited.'
WHERE website = 'https://www.itnnews.lk';

UPDATE source_endpoints AS endpoint
SET endpoint_type = 'rest_api',
    url = 'https://www.itnnews.lk/wp-json/wp/v2/posts?per_page=20&_fields=id,date,date_gmt,modified,modified_gmt,link,title,excerpt',
    polling_interval_seconds = 900,
    verified_official = true,
    paused = false,
    health_state = 'unknown',
    last_error = NULL,
    consecutive_failures = 0,
    backoff_until = NULL,
    etag = NULL,
    last_modified = NULL
FROM sources AS source
WHERE endpoint.source_id = source.id
  AND source.website = 'https://www.itnnews.lk';

UPDATE sources
SET active = true,
    archived_at = NULL,
    description = 'Official Sinhala digital news service of MTV Channel (Private) Limited.'
WHERE website = 'https://sinhala.newsfirst.lk';

WITH catalog (name, legal_name, source_type, website, description, icon_url) AS (
  VALUES
    ('Praja', 'Ajith Perakum Jayasinghe', 'independent', 'https://www.praja.lk', 'Registered independent Sinhala public-interest analysis and reporting.', 'https://praja.lk/wp-content/uploads/2025/03/cropped-logo-1.jpeg'),
    ('Yukthiya', 'C. J. Amarathunga', 'independent', 'https://www.yukthiya.lk', 'Registered independent Sinhala news and analysis publication.', 'https://www.yukthiya.lk/wp-content/uploads/2024/04/yicon1.png')
)
INSERT INTO sources (name, legal_name, source_type, website, active, description, icon_url)
SELECT name, legal_name, source_type, website, true, description, icon_url
FROM catalog
WHERE NOT EXISTS (
  SELECT 1 FROM sources WHERE sources.website = catalog.website AND archived_at IS NULL
);

WITH feeds (website, endpoint_type, url) AS (
  VALUES
    ('https://sinhala.newsfirst.lk', 'rest_api', 'https://apisinhala.newsfirst.lk/post/sticky'),
    ('https://www.praja.lk', 'rss', 'https://www.praja.lk/feed/'),
    ('https://www.yukthiya.lk', 'rest_api', 'https://www.yukthiya.lk/?rest_route=/wp/v2/posts&per_page=20&_fields=id,date,date_gmt,modified,modified_gmt,link,title,excerpt')
)
INSERT INTO source_endpoints (
  source_id, endpoint_type, url, polling_interval_seconds, verified_official, paused, health_state
)
SELECT source.id, feeds.endpoint_type, feeds.url, 900, true, false, 'unknown'
FROM feeds
JOIN sources AS source ON source.website = feeds.website AND source.archived_at IS NULL
WHERE NOT EXISTS (SELECT 1 FROM source_endpoints WHERE source_endpoints.url = feeds.url);

INSERT INTO rights_profiles (
  source_id, endpoint_id, mode, attribution, effective_from, review_on, approved_by, approved_at
)
SELECT source.id,
       endpoint.id,
       'discovery_only',
       'මූලාශ්‍රය: ' || source.name,
       clock_timestamp(),
       CURRENT_DATE + 180,
       'trusted-source-catalog',
       clock_timestamp()
FROM sources AS source
JOIN source_endpoints AS endpoint ON endpoint.source_id = source.id
WHERE endpoint.url IN (
  'https://apisinhala.newsfirst.lk/post/sticky',
  'https://www.praja.lk/feed/',
  'https://www.yukthiya.lk/?rest_route=/wp/v2/posts&per_page=20&_fields=id,date,date_gmt,modified,modified_gmt,link,title,excerpt'
)
AND NOT EXISTS (
  SELECT 1 FROM rights_profiles WHERE rights_profiles.endpoint_id = endpoint.id
);

COMMIT;
