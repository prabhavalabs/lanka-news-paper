BEGIN;

WITH catalog (name, legal_name, source_type, website, active, description, icon_url) AS (
  VALUES
    ('Gossip Lanka News', 'Yathura Media Networks (Private) Limited', 'private_media', 'https://www.gossiplankanews.com', true, 'Sinhala news, current affairs, and entertainment publication.', 'https://www.gossiplankanews.com/favicon.ico'),
    ('Meepura News', 'Meepura News', 'independent', 'https://www.meepura.com', true, 'Independent Sinhala regional news publication based in Negombo.', 'https://www.meepura.com/favicon.ico'),
    ('Info Sri Lanka', 'Info Sri Lanka', 'independent', 'https://www.infosrilanka.lk', true, 'Sinhala news, opinion, and current-affairs publication.', 'https://www.infosrilanka.lk/wp-content/uploads/2025/03/cropped-infosrilanka_favicon-192x192.webp'),
    ('Rata.lk', 'Rata.lk', 'independent', 'https://rata.lk', true, 'Sinhala national and regional news publication.', 'https://rata.lk/wp-content/uploads/2020/10/cropped-Logo-512-x-512-1-192x192.jpg'),
    ('Aithiya', 'Aithiya', 'independent', 'https://aithiya.lk', true, 'Independent Sinhala human-rights news publication.', 'https://aithiya.lk/wp-content/uploads/2020/03/cropped-512-192x192.jpg'),
    ('MediaLK', 'MediaLK', 'independent', 'https://medialk.com', true, 'Independent Sinhala investigative and public-interest news publication.', 'https://medialk.com/wp-content/uploads/2026/05/cropped-MediaLK.com-Logo-Portrait-2-1-192x192.png'),
    ('News 19', 'News 19', 'private_media', 'https://www.news19.lk', true, 'Sinhala breaking news and current-affairs publication.', 'https://www.news19.lk/wp-content/uploads/2020/11/Attachment-01-300x300.png'),
    ('Gossip Lanka Hot News', 'Gossip Lanka Hot News', 'private_media', 'https://www.gossiplankahotnews.com', false, 'Registered publisher; held because its official feed is stale.', 'https://www.gossiplankahotnews.com/favicon.ico'),
    ('Supiri Gossip', 'Supiri Gossip', 'private_media', 'https://supirigossip.com', false, 'Registered publisher; held because its official feed is stale.', NULL),
    ('Sarasaviya', 'Associated Newspapers of Ceylon Limited', 'state_owned', 'https://sarasaviya.lk', false, 'Registered publisher; held until an official ingestible Sinhala feed is available.', NULL),
    ('Presidential Media Division Sinhala', 'Presidential Media Division', 'government', 'https://pmd.gov.lk/si', false, 'Registered publisher; held until an official ingestible Sinhala feed is available.', NULL)
)
INSERT INTO sources (name, legal_name, source_type, website, active, description, icon_url)
SELECT name, legal_name, source_type, website, active, description, icon_url
FROM catalog
WHERE NOT EXISTS (
  SELECT 1 FROM sources WHERE sources.website = catalog.website AND archived_at IS NULL
);

WITH feeds (website, url, paused, health_state, last_error) AS (
  VALUES
    ('https://www.gossiplankanews.com', 'https://www.gossiplankanews.com/feeds/posts/default?alt=rss', false, 'unknown', NULL),
    ('https://www.meepura.com', 'https://www.meepura.com/feed/', false, 'unknown', NULL),
    ('https://www.infosrilanka.lk', 'https://www.infosrilanka.lk/feed/', false, 'unknown', NULL),
    ('https://rata.lk', 'https://rata.lk/feed/', false, 'unknown', NULL),
    ('https://aithiya.lk', 'https://aithiya.lk/feed/', false, 'unknown', NULL),
    ('https://medialk.com', 'https://medialk.com/feed', false, 'unknown', NULL),
    ('https://www.news19.lk', 'https://www.news19.lk/feed/', false, 'unknown', NULL),
    ('https://www.gossiplankahotnews.com', 'https://www.gossiplankahotnews.com/feeds/posts/default?alt=rss', true, 'stale', 'Official feed has not published since 2024-06-03'),
    ('https://supirigossip.com', 'https://supirigossip.com/feed/', true, 'stale', 'Official feed has not published since 2025-08-24')
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
  'https://www.gossiplankanews.com/feeds/posts/default?alt=rss',
  'https://www.meepura.com/feed/',
  'https://www.infosrilanka.lk/feed/',
  'https://rata.lk/feed/',
  'https://aithiya.lk/feed/',
  'https://medialk.com/feed',
  'https://www.news19.lk/feed/',
  'https://www.gossiplankahotnews.com/feeds/posts/default?alt=rss',
  'https://supirigossip.com/feed/'
)
AND NOT EXISTS (
  SELECT 1 FROM rights_profiles WHERE rights_profiles.endpoint_id = endpoint.id
);

COMMIT;
