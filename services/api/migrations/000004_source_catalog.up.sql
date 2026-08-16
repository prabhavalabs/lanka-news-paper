BEGIN;

UPDATE sources
SET active = false,
    description = 'Official publisher source; held because its former feed now redirects to a web page.'
WHERE website = 'https://www.itnnews.lk';

UPDATE source_endpoints AS endpoint
SET verified_official = false,
    paused = true,
    health_state = 'failed',
    last_error = 'Feed URL redirects to a web page instead of an RSS document'
FROM sources AS source
WHERE endpoint.source_id = source.id
  AND source.website = 'https://www.itnnews.lk';

UPDATE sources
SET active = true,
    description = 'Sinhala news published by Wijeya Newspapers.'
WHERE website = 'https://www.lankadeepa.lk';

UPDATE source_endpoints AS endpoint
SET url = 'https://www.lankadeepa.lk/rss/latest_news/1',
    polling_interval_seconds = 900,
    verified_official = true,
    paused = false,
    health_state = 'unknown',
    last_error = NULL
FROM sources AS source
WHERE endpoint.source_id = source.id
  AND source.website = 'https://www.lankadeepa.lk';

UPDATE sources
SET name = 'Ada Derana',
    legal_name = 'Power House Limited',
    website = 'https://sinhala.adaderana.lk',
    active = true,
    description = 'Ada Derana Sinhala breaking news service.'
WHERE website = 'https://www.ada.lk';

UPDATE source_endpoints AS endpoint
SET url = 'https://sinhala.adaderana.lk/rsshotnews.php',
    polling_interval_seconds = 900,
    verified_official = true,
    paused = false,
    health_state = 'unknown',
    last_error = NULL
FROM sources AS source
WHERE endpoint.source_id = source.id
  AND source.website = 'https://sinhala.adaderana.lk';

WITH catalog (name, legal_name, source_type, website, description) AS (
  VALUES
    ('Divaina', 'Upali Newspapers (Private) Limited', 'private_media', 'https://www.divaina.lk', 'Divaina Sinhala daily news.'),
    ('Vikalpa', 'Centre for Policy Alternatives', 'independent', 'https://www.vikalpa.org', 'Sinhala citizen journalism and public-interest reporting.'),
    ('Siyatha News', 'Voice of Asia Network (Private) Limited', 'private_media', 'https://siyathanews.lk', 'Siyatha television and digital news service.'),
    ('Sri Lanka Mirror Sinhala', 'Sri Lanka Mirror', 'private_media', 'https://sinhala.srilankamirror.com', 'Sinhala edition of Sri Lanka Mirror.'),
    ('Anidda', 'Anidda Newspaper', 'independent', 'https://www.anidda.lk', 'Independent Sinhala newspaper and news service.'),
    ('Dasatha Lanka News', 'Dasatha Lanka News (Private) Limited', 'private_media', 'https://dasathalankanews.com', 'Sinhala breaking news and current affairs.'),
    ('LankaSara', 'LankaSara', 'independent', 'https://lankasara.com/si', 'Sinhala news and current-affairs publication.')
)
INSERT INTO sources (name, legal_name, source_type, website, active, description)
SELECT name, legal_name, source_type, website, true, description
FROM catalog
WHERE NOT EXISTS (
  SELECT 1 FROM sources WHERE sources.website = catalog.website AND archived_at IS NULL
);

WITH feeds (website, url) AS (
  VALUES
    ('https://www.divaina.lk', 'https://www.divaina.lk/feed'),
    ('https://www.vikalpa.org', 'https://www.vikalpa.org/feed'),
    ('https://siyathanews.lk', 'https://siyathanews.lk/feed'),
    ('https://sinhala.srilankamirror.com', 'https://sinhala.srilankamirror.com/feed/'),
    ('https://www.anidda.lk', 'https://www.anidda.lk/feed/'),
    ('https://dasathalankanews.com', 'https://dasathalankanews.com/feed/'),
    ('https://lankasara.com/si', 'https://lankasara.com/si/feed/')
)
INSERT INTO source_endpoints (
  source_id, endpoint_type, url, polling_interval_seconds, verified_official, paused, health_state
)
SELECT source.id, 'rss', feeds.url, 900, true, false, 'unknown'
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
       'verified-source-catalog',
       clock_timestamp()
FROM sources AS source
JOIN source_endpoints AS endpoint ON endpoint.source_id = source.id
WHERE endpoint.url IN (
  'https://www.divaina.lk/feed',
  'https://www.vikalpa.org/feed',
  'https://siyathanews.lk/feed',
  'https://sinhala.srilankamirror.com/feed/',
  'https://www.anidda.lk/feed/',
  'https://dasathalankanews.com/feed/',
  'https://lankasara.com/si/feed/'
)
AND NOT EXISTS (
  SELECT 1 FROM rights_profiles WHERE rights_profiles.endpoint_id = endpoint.id
);

UPDATE rights_profiles AS rights
SET attribution = 'මූලාශ්‍රය: ' || source.name,
    review_on = COALESCE(rights.review_on, CURRENT_DATE + 180)
FROM sources AS source
WHERE rights.source_id = source.id
  AND source.website IN ('https://www.lankadeepa.lk', 'https://sinhala.adaderana.lk');

COMMIT;
