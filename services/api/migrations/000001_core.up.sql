BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE FUNCTION set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at = clock_timestamp();
  RETURN NEW;
END;
$$;

CREATE TABLE sources (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  legal_name text NOT NULL,
  source_type text NOT NULL,
  website text,
  country text NOT NULL DEFAULT 'LK',
  active boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  archived_at timestamptz,
  CONSTRAINT sources_type_valid CHECK (source_type IN (
    'private_media', 'state_owned', 'government', 'independent', 'international', 'other'
  ))
);

CREATE TRIGGER sources_set_updated_at
BEFORE UPDATE ON sources
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE source_endpoints (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_id uuid NOT NULL REFERENCES sources(id),
  endpoint_type text NOT NULL,
  url text NOT NULL,
  channel_id text,
  auth_ref text,
  polling_interval_seconds integer NOT NULL DEFAULT 300,
  timezone text NOT NULL DEFAULT 'Asia/Colombo',
  verified_official boolean NOT NULL DEFAULT false,
  health_state text NOT NULL DEFAULT 'unknown',
  paused boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT endpoints_type_valid CHECK (endpoint_type IN (
    'rss', 'atom', 'json_feed', 'rest_api', 'webhook', 'youtube'
  )),
  CONSTRAINT endpoints_interval_positive CHECK (polling_interval_seconds >= 60)
);

CREATE TRIGGER source_endpoints_set_updated_at
BEFORE UPDATE ON source_endpoints
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE rights_profiles (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_id uuid NOT NULL REFERENCES sources(id),
  endpoint_id uuid REFERENCES source_endpoints(id),
  version integer NOT NULL DEFAULT 1,
  mode text NOT NULL,
  commercial_use boolean NOT NULL DEFAULT false,
  allowed_public_fields text[] NOT NULL DEFAULT ARRAY['source_name','headline','published_at','canonical_url','category'],
  excerpt_max_characters integer NOT NULL DEFAULT 0,
  images_allowed boolean NOT NULL DEFAULT false,
  logo_allowed boolean NOT NULL DEFAULT false,
  video_embed_allowed boolean NOT NULL DEFAULT false,
  translation_allowed boolean NOT NULL DEFAULT false,
  automated_summary_allowed boolean NOT NULL DEFAULT false,
  raw_payload_retention_days integer NOT NULL DEFAULT 30,
  attribution text NOT NULL,
  effective_from timestamptz NOT NULL,
  expires_at timestamptz,
  review_on date,
  approved_by text,
  approved_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT rights_mode_valid CHECK (mode IN (
    'discovery_only', 'licensed_excerpt', 'licensed_media',
    'full_syndication', 'internal_verification', 'disabled'
  )),
  CONSTRAINT rights_version_positive CHECK (version > 0),
  UNIQUE (source_id, endpoint_id, version)
);

CREATE TABLE ingestion_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  endpoint_id uuid NOT NULL REFERENCES source_endpoints(id),
  started_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  ended_at timestamptz,
  status text NOT NULL DEFAULT 'running',
  http_status integer,
  item_count integer NOT NULL DEFAULT 0,
  new_item_count integer NOT NULL DEFAULT 0,
  error_detail text,
  CONSTRAINT ingestion_status_valid CHECK (status IN ('running', 'ok', 'failed', 'partial'))
);

CREATE TABLE categories (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug text NOT NULL UNIQUE,
  name_si text NOT NULL,
  name_en text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  CONSTRAINT categories_status_valid CHECK (status IN ('active', 'archived'))
);

CREATE TABLE articles (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_id uuid NOT NULL REFERENCES sources(id),
  endpoint_id uuid NOT NULL REFERENCES source_endpoints(id),
  rights_profile_id uuid NOT NULL REFERENCES rights_profiles(id),
  source_item_id text NOT NULL,
  original_url text NOT NULL,
  canonical_url text NOT NULL,
  headline text NOT NULL,
  description text,
  language text NOT NULL DEFAULT 'si',
  fingerprint text NOT NULL,
  published_at timestamptz NOT NULL,
  received_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  public_status text NOT NULL DEFAULT 'held',
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT articles_status_valid CHECK (public_status IN (
    'held', 'published', 'unpublished', 'removed', 'quarantined'
  )),
  UNIQUE (source_id, source_item_id),
  UNIQUE (fingerprint)
);

CREATE TRIGGER articles_set_updated_at
BEFORE UPDATE ON articles
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX articles_public_feed
  ON articles (published_at DESC, id DESC)
  WHERE public_status = 'published';

CREATE INDEX articles_headline_trgm
  ON articles USING gin (headline gin_trgm_ops);

CREATE TABLE article_versions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  article_id uuid NOT NULL REFERENCES articles(id),
  version integer NOT NULL,
  changed_fields jsonb NOT NULL DEFAULT '{}'::jsonb,
  source_updated_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  UNIQUE (article_id, version)
);

CREATE TABLE event_clusters (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  display_title text NOT NULL,
  category_id uuid REFERENCES categories(id),
  confidence numeric,
  status text NOT NULL DEFAULT 'open',
  first_seen_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  last_update_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  locked boolean NOT NULL DEFAULT false,
  CONSTRAINT clusters_status_valid CHECK (status IN ('open', 'closed', 'merged'))
);

CREATE TABLE event_articles (
  event_id uuid NOT NULL REFERENCES event_clusters(id),
  article_id uuid NOT NULL REFERENCES articles(id),
  clustering_score numeric,
  origin text NOT NULL DEFAULT 'automatic',
  PRIMARY KEY (event_id, article_id),
  CONSTRAINT event_articles_origin_valid CHECK (origin IN ('automatic', 'manual'))
);

CREATE TABLE editorial_actions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_id uuid,
  entity_type text NOT NULL,
  entity_id uuid NOT NULL,
  action text NOT NULL,
  before_value jsonb,
  after_value jsonb,
  reason text,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE complaints (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  requester_name text,
  requester_contact text,
  entity_type text NOT NULL,
  entity_id uuid NOT NULL,
  reason text NOT NULL,
  evidence text,
  status text NOT NULL DEFAULT 'open',
  owner_id uuid,
  resolution text,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  resolved_at timestamptz,
  CONSTRAINT complaints_status_valid CHECK (status IN ('open', 'in_review', 'resolved', 'rejected'))
);

CREATE TABLE admin_users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email citext NOT NULL UNIQUE,
  display_name text NOT NULL,
  role text NOT NULL,
  mfa_enabled boolean NOT NULL DEFAULT false,
  status text NOT NULL DEFAULT 'invited',
  last_login_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT admin_users_role_valid CHECK (role IN (
    'administrator', 'source_manager', 'editor', 'compliance_reviewer', 'operations_engineer'
  )),
  CONSTRAINT admin_users_status_valid CHECK (status IN ('invited', 'active', 'suspended'))
);

CREATE TABLE audit_logs (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  actor_id uuid,
  action text NOT NULL,
  target_type text NOT NULL,
  target_id text NOT NULL,
  ip inet,
  result text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

INSERT INTO categories (slug, name_si, name_en) VALUES
  ('latest', 'නවතම', 'Latest'),
  ('politics', 'දේශපාලන', 'Politics'),
  ('economy', 'ආර්ථික', 'Economy'),
  ('local', 'දේශීය', 'Local'),
  ('world', 'ලෝක', 'World'),
  ('sport', 'ක්‍රීඩා', 'Sport'),
  ('technology', 'තාක්ෂණ', 'Technology'),
  ('health', 'සෞඛ්‍ය', 'Health'),
  ('environment', 'පරිසර', 'Environment'),
  ('crime', 'අපරාධ', 'Crime'),
  ('education', 'අධ්‍යාපන', 'Education'),
  ('entertainment', 'විනෝදාස්වාදය', 'Entertainment'),
  ('official', 'නිල නිවේදන', 'Official announcements');

COMMIT;
