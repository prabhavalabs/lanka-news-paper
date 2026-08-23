BEGIN;

-- The operator has approved private, educational analysis for these sources.
-- Full text remains private: public full text, embeddings, and training stay disabled.
CREATE TEMP TABLE permitted_article_sources (
  website text PRIMARY KEY,
  article_method text NOT NULL,
  min_content_characters integer NOT NULL,
  request_timeout_seconds integer NOT NULL,
  allowed_hosts jsonb NOT NULL,
  robots_url text NOT NULL,
  terms_urls text[] NOT NULL DEFAULT '{}'
) ON COMMIT DROP;

INSERT INTO permitted_article_sources (
  website, article_method, min_content_characters,
  request_timeout_seconds, allowed_hosts, robots_url, terms_urls
) VALUES
  ('https://aithiya.lk', 'html_static', 120, 15, '["aithiya.lk"]', 'https://aithiya.lk/robots.txt', '{}'),
  ('https://www.anidda.lk', 'html_static', 120, 15, '["www.anidda.lk"]', 'https://www.anidda.lk/robots.txt', '{}'),
  ('https://www.bbc.com/sinhala', 'html_static', 120, 15, '["www.bbc.com","feeds.bbci.co.uk"]', 'https://www.bbc.com/robots.txt', '{}'),
  ('https://dasathalankanews.com', 'html_static', 120, 15, '["dasathalankanews.com"]', 'https://dasathalankanews.com/robots.txt', '{}'),
  ('https://www.divaina.lk', 'html_static', 120, 15, '["www.divaina.lk"]', 'https://www.divaina.lk/robots.txt', '{}'),
  ('https://www.gossiplankanews.com', 'feed_content', 100, 15, '["www.gossiplankanews.com"]', 'https://www.gossiplankanews.com/robots.txt', ARRAY['https://cdn.gossiplankanews.com/about/aboutus.html','https://cdn.gossiplankanews.com/about/contactus.html']),
  ('https://www.itnnews.lk', 'api_content', 100, 20, '["www.itnnews.lk"]', 'https://www.itnnews.lk/robots.txt', '{}'),
  ('https://www.infosrilanka.lk', 'html_static', 120, 15, '["www.infosrilanka.lk"]', 'https://www.infosrilanka.lk/robots.txt', '{}'),
  ('https://sinhala.lankanewsweb.net', 'html_static', 120, 30, '["sinhala.lankanewsweb.net"]', 'https://sinhala.lankanewsweb.net/robots.txt', '{}'),
  ('https://www.lankadeepa.lk', 'feed_content', 60, 20, '["www.lankadeepa.lk"]', 'https://www.lankadeepa.lk/robots.txt', ARRAY['https://www.lankadeepa.lk/rss']),
  ('https://medialk.com', 'html_static', 120, 15, '["medialk.com"]', 'https://medialk.com/robots.txt', '{}'),
  ('https://www.meepura.com', 'html_static', 120, 15, '["www.meepura.com"]', 'https://www.meepura.com/robots.txt', '{}'),
  ('https://www.news19.lk', 'html_static', 120, 30, '["www.news19.lk"]', 'https://www.news19.lk/robots.txt', '{}'),
  ('https://sinhala.newsfirst.lk', 'api_content', 100, 20, '["apisinhala.newsfirst.lk","sinhala.newsfirst.lk"]', 'https://sinhala.newsfirst.lk/robots.txt', '{}'),
  ('https://praja.lk', 'html_static', 120, 15, '["praja.lk"]', 'https://praja.lk/robots.txt', '{}'),
  ('https://siyathanews.lk', 'html_static', 120, 15, '["siyathanews.lk"]', 'https://siyathanews.lk/robots.txt', '{}'),
  ('https://sinhala.srilankamirror.com', 'html_static', 120, 30, '["sinhala.srilankamirror.com"]', 'https://sinhala.srilankamirror.com/robots.txt', '{}'),
  ('https://www.vikalpa.org', 'html_static', 120, 15, '["www.vikalpa.org"]', 'https://www.vikalpa.org/robots.txt', '{}');

INSERT INTO sources (
  name, legal_name, source_type, website, active, description, icon_url
)
SELECT 'Gossip Lanka News',
       'Yathura Media Networks (Private) Limited',
       'private_media',
       'https://www.gossiplankanews.com',
       true,
       'Sinhala news, current affairs, and entertainment publication.',
       'https://www.gossiplankanews.com/favicon.ico'
WHERE NOT EXISTS (
  SELECT 1 FROM sources WHERE website = 'https://www.gossiplankanews.com'
);

UPDATE sources
SET active = true,
    archived_at = NULL,
    description = 'Sinhala news, current affairs, and entertainment publication.'
WHERE website = 'https://www.gossiplankanews.com';

INSERT INTO source_endpoints (
  source_id, endpoint_type, url, polling_interval_seconds,
  verified_official, paused, health_state
)
SELECT source.id,
       'rss',
       'https://www.gossiplankanews.com/feeds/posts/default?alt=rss',
       300,
       true,
       false,
       'unknown'
FROM sources source
WHERE source.website = 'https://www.gossiplankanews.com'
  AND NOT EXISTS (
    SELECT 1 FROM source_endpoints endpoint WHERE endpoint.source_id = source.id
  );

UPDATE source_endpoints endpoint
SET endpoint_type = 'rss',
    url = 'https://www.gossiplankanews.com/feeds/posts/default?alt=rss'
FROM sources source
WHERE endpoint.source_id = source.id
  AND source.website = 'https://www.gossiplankanews.com';

UPDATE source_endpoints endpoint
SET endpoint_type = 'rss',
    url = 'https://www.lankadeepa.lk/rss/latest_news/1'
FROM sources source
WHERE endpoint.source_id = source.id
  AND source.website = 'https://www.lankadeepa.lk';

UPDATE source_endpoints endpoint
SET endpoint_type = 'rest_api',
    url = 'https://www.itnnews.lk/wp-json/wp/v2/posts?per_page=20&_fields=id,date,date_gmt,modified,modified_gmt,link,title,excerpt,content'
FROM sources source
WHERE endpoint.source_id = source.id
  AND source.website = 'https://www.itnnews.lk';

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
FROM sources source
JOIN permitted_article_sources permitted ON permitted.website = source.website
WHERE endpoint.source_id = source.id;

-- Ensure Gossip Lanka has a baseline rights row before creating the permanent
-- private-research version below.
INSERT INTO rights_profiles (
  source_id, endpoint_id, version, mode, attribution, effective_from,
  review_on, approved_by, approved_at
)
SELECT source.id,
       endpoint.id,
       1,
       'discovery_only',
       'මූලාශ්‍රය: ' || source.name,
       clock_timestamp(),
       CURRENT_DATE + 90,
       'migration-000038',
       clock_timestamp()
FROM sources source
JOIN source_endpoints endpoint ON endpoint.source_id = source.id
WHERE source.website = 'https://www.gossiplankanews.com'
  AND NOT EXISTS (
    SELECT 1 FROM rights_profiles rights WHERE rights.endpoint_id = endpoint.id
  );

UPDATE source_collection_profiles profile
SET active = false
FROM source_endpoints endpoint
JOIN sources source ON source.id = endpoint.source_id
JOIN permitted_article_sources permitted ON permitted.website = source.website
WHERE profile.endpoint_id = endpoint.id
  AND profile.active;

INSERT INTO source_collection_profiles (
  source_id, endpoint_id, version, active, discovery_method, article_method,
  config, min_delay_seconds, max_requests_per_run, max_pages,
  request_timeout_seconds, created_by, activated_at
)
SELECT source.id,
       endpoint.id,
       COALESCE(history.max_version, 0) + 1,
       true,
       endpoint.endpoint_type,
       permitted.article_method,
       jsonb_build_object(
         'discovery_urls', jsonb_build_array(endpoint.url),
         'allowed_hosts', permitted.allowed_hosts,
         'article_url_patterns', '[]'::jsonb,
         'content_selector', '',
         'exclude_selectors', jsonb_build_array(
           'script', 'style', 'nav', 'header', 'footer', 'aside',
           '.advertisement', '.ads', '.related-posts', '.social-share'
         ),
         'pagination_mode', 'none',
         'user_agent', 'SNAPBot/1.0',
         'min_content_characters', permitted.min_content_characters,
         'minimum_sinhala_ratio', 0
       ),
       5,
       25,
       1,
       permitted.request_timeout_seconds,
       'migration-000038',
       clock_timestamp()
FROM source_endpoints endpoint
JOIN sources source ON source.id = endpoint.source_id
JOIN permitted_article_sources permitted ON permitted.website = source.website
LEFT JOIN LATERAL (
  SELECT max(profile.version) AS max_version
  FROM source_collection_profiles profile
  WHERE profile.endpoint_id = endpoint.id
) history ON true;

UPDATE source_compliance_reviews review
SET active = false
FROM sources source
JOIN permitted_article_sources permitted ON permitted.website = source.website
WHERE review.source_id = source.id
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
       permitted.robots_url,
       clock_timestamp(),
       true,
       permitted.terms_urls,
       true,
       true,
       true,
       false,
       false,
       false,
       'Operator-approved private educational analysis. Discovery and private full-text processing are allowed. Public full text, embeddings, and model training remain disabled.',
       'operator:user-approved',
       clock_timestamp(),
       CURRENT_DATE + 90
FROM sources source
JOIN permitted_article_sources permitted ON permitted.website = source.website
LEFT JOIN LATERAL (
  SELECT max(review.version) AS max_version
  FROM source_compliance_reviews review
  WHERE review.source_id = source.id
) history ON true;

CREATE TEMP TABLE migration_000038_rights (
  endpoint_id uuid PRIMARY KEY,
  rights_id uuid NOT NULL
) ON COMMIT DROP;

WITH inserted AS (
  INSERT INTO rights_profiles (
    source_id, endpoint_id, version, mode, commercial_use,
    allowed_public_fields, excerpt_max_characters, images_allowed,
    logo_allowed, video_embed_allowed, translation_allowed,
    automated_summary_allowed, raw_payload_retention_days, attribution,
    effective_from, expires_at, review_on, approved_by, approved_at
  )
  SELECT source.id,
         endpoint.id,
         latest.version + 1,
         latest.mode,
         latest.commercial_use,
         latest.allowed_public_fields,
         latest.excerpt_max_characters,
         latest.images_allowed,
         latest.logo_allowed,
         latest.video_embed_allowed,
         latest.translation_allowed,
         latest.automated_summary_allowed,
         0,
         latest.attribution,
         clock_timestamp(),
         NULL,
         CURRENT_DATE + 90,
         'operator:user-approved-private-research',
         clock_timestamp()
  FROM source_endpoints endpoint
  JOIN sources source ON source.id = endpoint.source_id
  JOIN permitted_article_sources permitted ON permitted.website = source.website
  JOIN LATERAL (
    SELECT rights.*
    FROM rights_profiles rights
    WHERE rights.endpoint_id = endpoint.id
    ORDER BY rights.version DESC
    LIMIT 1
  ) latest ON true
  RETURNING endpoint_id, id
)
INSERT INTO migration_000038_rights (endpoint_id, rights_id)
SELECT endpoint_id, id FROM inserted;

UPDATE articles article
SET rights_profile_id = migration.rights_id
FROM migration_000038_rights migration
WHERE article.endpoint_id = migration.endpoint_id;

UPDATE article_contents content
SET retention_until = NULL
FROM articles article
JOIN source_endpoints endpoint ON endpoint.id = article.endpoint_id
JOIN sources source ON source.id = endpoint.source_id
JOIN permitted_article_sources permitted ON permitted.website = source.website
WHERE content.article_id = article.id
  AND content.current;

COMMIT;
