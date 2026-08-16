BEGIN;

ALTER TABLE admin_users
  ADD COLUMN password_hash text,
  ADD COLUMN totp_secret text;

CREATE TABLE admin_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES admin_users(id),
  token_hash text NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

ALTER TABLE source_endpoints
  ADD COLUMN etag text,
  ADD COLUMN last_modified text,
  ADD COLUMN last_success_at timestamptz,
  ADD COLUMN last_error text;

ALTER TABLE articles
  ADD COLUMN category_id uuid REFERENCES categories(id),
  ADD COLUMN publisher_category text,
  ADD COLUMN event_id uuid REFERENCES event_clusters(id);

ALTER TABLE event_clusters
  ADD COLUMN is_breaking boolean NOT NULL DEFAULT false;

CREATE TABLE daily_briefs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brief_date date NOT NULL UNIQUE,
  title_si text NOT NULL,
  body_si text NOT NULL,
  model text,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE llm_providers (
  id text PRIMARY KEY,
  kind text NOT NULL,
  base_url text,
  api_key_ref text,
  enabled boolean NOT NULL DEFAULT false,
  status text NOT NULL DEFAULT 'unknown',
  status_detail text,
  checked_at timestamptz,
  CONSTRAINT llm_providers_kind_valid CHECK (kind IN ('codex_cli', 'openai_api'))
);

CREATE TABLE llm_task_profiles (
  task text NOT NULL,
  priority integer NOT NULL,
  provider_id text NOT NULL REFERENCES llm_providers(id),
  model text NOT NULL,
  reasoning_effort text,
  max_output_tokens integer,
  temperature numeric,
  timeout_seconds integer NOT NULL DEFAULT 60,
  enabled boolean NOT NULL DEFAULT true,
  PRIMARY KEY (task, priority)
);

CREATE TABLE llm_calls (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  task text,
  provider_id text,
  model text,
  input_tokens integer,
  output_tokens integer,
  latency_ms integer,
  outcome text,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

INSERT INTO sources (name, legal_name, source_type, website, active) VALUES
  ('BBC News Sinhala', 'British Broadcasting Corporation', 'international', 'https://www.bbc.com/sinhala', true),
  ('ITN News', 'Independent Television Network Ltd', 'state_owned', 'https://www.itnnews.lk', true),
  ('Lankadeepa', 'Wijeya Newspapers Ltd', 'private_media', 'https://www.lankadeepa.lk', false),
  ('Ada.lk', 'Ada Derana / TV Derana', 'private_media', 'https://www.ada.lk', false);

INSERT INTO source_endpoints (source_id, endpoint_type, url, verified_official, paused, health_state)
SELECT id, 'rss', 'https://feeds.bbci.co.uk/sinhala/rss.xml', true, false, 'unknown'
FROM sources WHERE name = 'BBC News Sinhala';

INSERT INTO source_endpoints (source_id, endpoint_type, url, verified_official, paused, health_state)
SELECT id, 'rss', 'https://www.itnnews.lk/feed/', true, false, 'unknown'
FROM sources WHERE name = 'ITN News';

INSERT INTO source_endpoints (source_id, endpoint_type, url, verified_official, paused, health_state)
SELECT id, 'rss', 'https://www.lankadeepa.lk/rss', false, true, 'unknown'
FROM sources WHERE name = 'Lankadeepa';

INSERT INTO source_endpoints (source_id, endpoint_type, url, verified_official, paused, health_state)
SELECT id, 'rss', 'https://www.ada.lk/rss', false, true, 'unknown'
FROM sources WHERE name = 'Ada.lk';

INSERT INTO rights_profiles (
  source_id, endpoint_id, mode, attribution, effective_from, approved_by, approved_at
)
SELECT s.id, e.id, 'discovery_only', 'මූලාශ්‍රය: ' || s.name, clock_timestamp(), 'bootstrap', clock_timestamp()
FROM sources s
JOIN source_endpoints e ON e.source_id = s.id;

COMMIT;
