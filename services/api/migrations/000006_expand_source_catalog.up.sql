BEGIN;

WITH catalog (name, legal_name, source_type, website, active, description, icon_url) AS (
  VALUES
    ('Mawrata News', 'Free Media Independent Networks (Private) Limited', 'private_media', 'https://mawratanews.lk', true, 'Sinhala news and current-affairs publication.', 'https://mawratanews.lk/wp-content/uploads/2023/07/cropped-icon-192x192.png'),
    ('Lanka C News', 'Lanka C News', 'independent', 'https://www.lankacnews.com', true, 'Sinhala breaking news and current-affairs publication.', 'https://www.lankacnews.com/favicon.ico'),
    ('ColomboXNews', 'ColomboXNews', 'independent', 'https://www.colomboxnews.com', true, 'Sinhala news, politics, and current-affairs publication.', NULL),
    ('LNW Sinhala', 'Lanka News Web', 'independent', 'https://sinhala.lankanewsweb.net', true, 'Sinhala edition of Lanka News Web.', NULL),
    ('Hiru News', 'Asia Broadcasting Corporation (Private) Limited', 'private_media', 'https://hirunews.lk', false, 'Registered publisher; held until an official ingestible Sinhala feed is available.', 'https://hirunews.lk/revampDesign/assets/favicon.ico'),
    ('Newsfirst Sinhala', 'MTV Channel (Private) Limited', 'private_media', 'https://sinhala.newsfirst.lk', false, 'Registered publisher; held until an official ingestible Sinhala feed is available.', 'https://cdn.newsfirst.lk/assets/favicon.png'),
    ('Dinamina', 'Associated Newspapers of Ceylon Limited', 'state_owned', 'https://www.dinamina.lk', false, 'Registered publisher; held until an official ingestible Sinhala feed is available.', NULL),
    ('Silumina', 'Associated Newspapers of Ceylon Limited', 'state_owned', 'https://www.silumina.lk', false, 'Registered publisher; held until an official ingestible Sinhala feed is available.', NULL),
    ('Mawbima', 'Ceylon Newspapers (Private) Limited', 'private_media', 'https://mawbima.lk', false, 'Registered publisher; held until an official ingestible Sinhala feed is available.', NULL),
    ('Aruna', 'Liberty Publishers (Private) Limited', 'private_media', 'https://aruna.lk', false, 'Registered publisher; held until an official ingestible Sinhala feed is available.', 'https://www.aruna.lk/assets/favicon.webp'),
    ('Monara', 'Monara', 'private_media', 'https://monara.com', false, 'Registered publisher; held until an official ingestible Sinhala feed is available.', NULL),
    ('Neth News', 'Asset Radio Broadcasting (Private) Limited', 'private_media', 'https://nethnews.lk', false, 'Registered publisher; held until an official ingestible Sinhala feed is available.', NULL),
    ('LankaTruth', 'LankaTruth', 'independent', 'https://lankatruth.com/si', false, 'Registered publisher; held until an official ingestible Sinhala feed is available.', NULL),
    ('NewsWire Sinhala', 'NewsWire', 'private_media', 'https://sinhala.newswire.lk', false, 'Registered publisher; held because its official feed is stale.', 'https://sinhala.newswire.lk/wp-content/uploads/2020/05/favicon.png'),
    ('Asian Mirror Sinhala', 'Asian Mirror', 'private_media', 'https://sinhala.asianmirror.lk', false, 'Registered publisher; held because its official feed currently contains no items.', 'https://sinhala.asianmirror.lk/wp-content/uploads/2022/03/cropped-icon-32x32.png'),
    ('Government News Sinhala', 'Department of Government Information', 'government', 'https://sinhala.news.lk', false, 'Registered publisher; held because its legacy official feed is stale.', 'https://sinhala.news.lk/images/newsfav.png'),
    ('Lankapuvath Sinhala', 'Lankapuvath Limited', 'state_owned', 'https://sinhala.lankapuvath.lk', false, 'Registered publisher; held until an official ingestible Sinhala feed is available.', NULL)
)
INSERT INTO sources (name, legal_name, source_type, website, active, description, icon_url)
SELECT name, legal_name, source_type, website, active, description, icon_url
FROM catalog
WHERE NOT EXISTS (
  SELECT 1 FROM sources WHERE sources.website = catalog.website AND archived_at IS NULL
);

WITH feeds (website, url, paused, health_state, last_error) AS (
  VALUES
    ('https://mawratanews.lk', 'https://mawratanews.lk/feed/', false, 'unknown', NULL),
    ('https://www.lankacnews.com', 'https://www.lankacnews.com/feeds/posts/default?alt=rss', false, 'unknown', NULL),
    ('https://www.colomboxnews.com', 'https://www.colomboxnews.com/feed/', false, 'unknown', NULL),
    ('https://sinhala.lankanewsweb.net', 'https://sinhala.lankanewsweb.net/feed/', false, 'unknown', NULL),
    ('https://sinhala.newswire.lk', 'https://sinhala.newswire.lk/feed/', true, 'stale', 'Official feed has not published since 2025-01-08'),
    ('https://sinhala.asianmirror.lk', 'https://sinhala.asianmirror.lk/feed/', true, 'stale', 'Official feed currently contains no items'),
    ('https://sinhala.news.lk', 'https://v3sin.news.lk/fetures/itemlist/category/6-news?format=feed&type=rss', true, 'stale', 'Legacy official feed has not published since 2024-12-19')
)
INSERT INTO source_endpoints (
  source_id, endpoint_type, url, polling_interval_seconds, verified_official, paused, health_state, last_error
)
SELECT source.id, 'rss', feeds.url, 900, true, feeds.paused, feeds.health_state, feeds.last_error
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
  'https://mawratanews.lk/feed/',
  'https://www.lankacnews.com/feeds/posts/default?alt=rss',
  'https://www.colomboxnews.com/feed/',
  'https://sinhala.lankanewsweb.net/feed/',
  'https://sinhala.newswire.lk/feed/',
  'https://sinhala.asianmirror.lk/feed/',
  'https://v3sin.news.lk/fetures/itemlist/category/6-news?format=feed&type=rss'
)
AND NOT EXISTS (
  SELECT 1 FROM rights_profiles WHERE rights_profiles.endpoint_id = endpoint.id
);

COMMIT;
