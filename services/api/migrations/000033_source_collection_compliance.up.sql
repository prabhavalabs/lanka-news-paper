BEGIN;

CREATE TABLE source_collection_profiles (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_id uuid NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
  endpoint_id uuid NOT NULL REFERENCES source_endpoints(id) ON DELETE CASCADE,
  version integer NOT NULL,
  active boolean NOT NULL DEFAULT false,
  discovery_method text NOT NULL,
  article_method text NOT NULL DEFAULT 'metadata_only',
  config jsonb NOT NULL DEFAULT '{}'::jsonb,
  min_delay_seconds integer NOT NULL DEFAULT 5,
  max_requests_per_run integer NOT NULL DEFAULT 25,
  max_pages integer NOT NULL DEFAULT 3,
  request_timeout_seconds integer NOT NULL DEFAULT 15,
  created_by text NOT NULL,
  activated_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT source_collection_version_positive CHECK (version > 0),
  CONSTRAINT source_collection_config_object CHECK (jsonb_typeof(config) = 'object'),
  CONSTRAINT source_collection_discovery_method_valid CHECK (discovery_method IN (
    'rss', 'atom', 'json_feed', 'rest_api', 'sitemap', 'listing_page', 'webhook', 'youtube'
  )),
  CONSTRAINT source_collection_article_method_valid CHECK (article_method IN (
    'metadata_only', 'feed_content', 'api_content', 'html_static'
  )),
  CONSTRAINT source_collection_limits_valid CHECK (
    min_delay_seconds BETWEEN 1 AND 86400
    AND max_requests_per_run BETWEEN 1 AND 500
    AND max_pages BETWEEN 1 AND 100
    AND request_timeout_seconds BETWEEN 3 AND 60
  ),
  UNIQUE (endpoint_id, version)
);

CREATE UNIQUE INDEX source_collection_one_active_per_endpoint
  ON source_collection_profiles (endpoint_id)
  WHERE active;

CREATE INDEX source_collection_source_history
  ON source_collection_profiles (source_id, endpoint_id, version DESC);

CREATE TABLE source_compliance_reviews (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_id uuid NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
  version integer NOT NULL,
  active boolean NOT NULL DEFAULT false,
  status text NOT NULL DEFAULT 'pending',
  robots_url text,
  robots_checked_at timestamptz,
  robots_allowed boolean,
  terms_urls text[] NOT NULL DEFAULT '{}',
  allow_discovery boolean NOT NULL DEFAULT false,
  allow_full_text_storage boolean NOT NULL DEFAULT false,
  allow_ai_processing boolean NOT NULL DEFAULT false,
  allow_embeddings boolean NOT NULL DEFAULT false,
  allow_training boolean NOT NULL DEFAULT false,
  allow_public_full_text boolean NOT NULL DEFAULT false,
  notes text NOT NULL DEFAULT '',
  reviewed_by text NOT NULL,
  reviewed_at timestamptz,
  review_on date,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT source_compliance_version_positive CHECK (version > 0),
  CONSTRAINT source_compliance_status_valid CHECK (status IN (
    'pending', 'approved', 'restricted', 'denied'
  )),
  CONSTRAINT source_compliance_denied_has_no_permissions CHECK (
    status <> 'denied' OR NOT (
      allow_discovery OR allow_full_text_storage OR allow_ai_processing
      OR allow_embeddings OR allow_training OR allow_public_full_text
    )
  ),
  CONSTRAINT source_compliance_full_text_dependencies CHECK (
    (NOT allow_public_full_text OR allow_full_text_storage)
    AND (NOT allow_embeddings OR allow_ai_processing)
    AND (NOT allow_training OR allow_ai_processing)
  ),
  UNIQUE (source_id, version)
);

CREATE UNIQUE INDEX source_compliance_one_active_per_source
  ON source_compliance_reviews (source_id)
  WHERE active;

CREATE INDEX source_compliance_review_due
  ON source_compliance_reviews (review_on)
  WHERE active AND review_on IS NOT NULL;

CREATE TABLE article_contents (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  article_id uuid NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  version integer NOT NULL,
  current boolean NOT NULL DEFAULT true,
  body_text text NOT NULL,
  acquisition_method text NOT NULL,
  source_url text NOT NULL,
  content_hash text NOT NULL,
  extractor_version text NOT NULL,
  collection_profile_id uuid NOT NULL REFERENCES source_collection_profiles(id),
  compliance_review_id uuid NOT NULL REFERENCES source_compliance_reviews(id),
  fetched_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  retention_until timestamptz,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT article_contents_version_positive CHECK (version > 0),
  CONSTRAINT article_contents_body_not_blank CHECK (length(btrim(body_text)) > 0),
  CONSTRAINT article_contents_method_valid CHECK (acquisition_method IN (
    'feed_content', 'api_content', 'html_static'
  )),
  UNIQUE (article_id, version)
);

CREATE UNIQUE INDEX article_contents_one_current
  ON article_contents (article_id)
  WHERE current;

CREATE INDEX article_contents_retention
  ON article_contents (retention_until)
  WHERE current AND retention_until IS NOT NULL;

CREATE TABLE crawl_attempts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  article_id uuid REFERENCES articles(id) ON DELETE CASCADE,
  source_id uuid NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
  collection_profile_id uuid REFERENCES source_collection_profiles(id),
  requested_url text NOT NULL,
  final_url text,
  status text NOT NULL DEFAULT 'running',
  http_status integer,
  response_bytes integer,
  duration_ms integer,
  extractor text,
  extracted_characters integer NOT NULL DEFAULT 0,
  error_detail text,
  started_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  finished_at timestamptz,
  CONSTRAINT crawl_attempt_status_valid CHECK (status IN (
    'running', 'succeeded', 'skipped', 'failed', 'blocked'
  ))
);

CREATE INDEX crawl_attempts_source_recent
  ON crawl_attempts (source_id, started_at DESC);

CREATE TABLE crawl_domain_leases (
  host text PRIMARY KEY,
  last_request_at timestamptz NOT NULL
);

INSERT INTO source_collection_profiles (
  source_id, endpoint_id, version, active, discovery_method, article_method,
  config, created_by, activated_at
)
SELECT endpoint.source_id,
       endpoint.id,
       1,
       true,
       endpoint.endpoint_type,
       CASE endpoint.endpoint_type
         WHEN 'rest_api' THEN 'api_content'
         WHEN 'rss' THEN 'feed_content'
         WHEN 'atom' THEN 'feed_content'
         WHEN 'json_feed' THEN 'feed_content'
         ELSE 'metadata_only'
       END,
       jsonb_build_object(
         'discovery_urls', jsonb_build_array(endpoint.url),
         'allowed_hosts', jsonb_build_array(lower(split_part(split_part(endpoint.url, '://', 2), '/', 1))),
         'article_url_patterns', '[]'::jsonb,
         'exclude_selectors', '[]'::jsonb,
         'pagination_mode', 'none'
       ),
       'migration',
       clock_timestamp()
FROM source_endpoints endpoint;

INSERT INTO source_compliance_reviews (
  source_id, version, active, status, robots_url, allow_discovery,
  allow_full_text_storage, allow_ai_processing, allow_embeddings,
  allow_training, allow_public_full_text, notes, reviewed_by,
  reviewed_at, review_on
)
SELECT source.id,
       1,
       true,
       'restricted',
       CASE
         WHEN source.website LIKE 'https://%'
         THEN regexp_replace(source.website, '^(https://[^/]+).*$', '\1/robots.txt')
         ELSE NULL
       END,
       EXISTS (
         SELECT 1
         FROM source_endpoints endpoint
         JOIN LATERAL (
           SELECT mode
           FROM rights_profiles rights
           WHERE rights.endpoint_id = endpoint.id
           ORDER BY rights.version DESC
           LIMIT 1
         ) latest_rights ON true
         WHERE endpoint.source_id = source.id
           AND latest_rights.mode NOT IN ('disabled', 'internal_verification')
       ),
       false,
       false,
       false,
       false,
       false,
       'Migrated conservatively. Full-text storage, crawling, and AI use require documented approval.',
       'migration',
       clock_timestamp(),
       (clock_timestamp() + interval '90 days')::date
FROM sources source
WHERE source.archived_at IS NULL;

COMMIT;
