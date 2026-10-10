BEGIN;

CREATE TABLE social_settings (
  singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
  enabled boolean NOT NULL DEFAULT false,
  auto_publish boolean NOT NULL DEFAULT false,
  max_posts_per_hour integer NOT NULL DEFAULT 2 CHECK (max_posts_per_hour BETWEEN 1 AND 100),
  max_posts_per_day integer NOT NULL DEFAULT 12 CHECK (max_posts_per_day BETWEEN 1 AND 1000),
  min_gap_minutes integer NOT NULL DEFAULT 20 CHECK (min_gap_minutes BETWEEN 1 AND 1440),
  quiet_hours_start smallint NOT NULL DEFAULT 0 CHECK (quiet_hours_start BETWEEN 0 AND 23),
  quiet_hours_end smallint NOT NULL DEFAULT 6 CHECK (quiet_hours_end BETWEEN 0 AND 23),
  min_score integer NOT NULL DEFAULT 70 CHECK (min_score BETWEEN 0 AND 100),
  lookback_hours integer NOT NULL DEFAULT 6 CHECK (lookback_hours BETWEEN 1 AND 168),
  candidates_per_run integer NOT NULL DEFAULT 8 CHECK (candidates_per_run BETWEEN 1 AND 30),
  include_single_articles boolean NOT NULL DEFAULT false,
  timezone text NOT NULL DEFAULT 'Asia/Colombo',
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  updated_by uuid REFERENCES admin_users(id) ON DELETE SET NULL
);
INSERT INTO social_settings (singleton) VALUES (true);

CREATE TABLE social_accounts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  platform text NOT NULL DEFAULT 'facebook_page' CHECK (platform = 'facebook_page'),
  page_id text NOT NULL UNIQUE,
  page_name text NOT NULL,
  page_picture_url text NOT NULL DEFAULT '',
  token_ciphertext bytea,
  token_nonce bytea,
  key_version text,
  granted_scopes text[] NOT NULL DEFAULT '{}',
  status text NOT NULL CHECK (status IN ('connected', 'needs_reconnect', 'disconnected')),
  connected_by uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  connected_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  last_verified_at timestamptz,
  token_expires_at timestamptz,
  last_error text,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CHECK ((token_ciphertext IS NULL) = (token_nonce IS NULL)),
  CHECK (status <> 'connected' OR (token_ciphertext IS NOT NULL AND key_version IS NOT NULL))
);
CREATE UNIQUE INDEX social_one_connected_account ON social_accounts ((true)) WHERE status = 'connected';
CREATE TRIGGER social_accounts_set_updated_at BEFORE UPDATE ON social_accounts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE social_oauth_states (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  state_hash bytea NOT NULL UNIQUE CHECK (octet_length(state_hash) = 32),
  admin_user_id uuid NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
  expires_at timestamptz NOT NULL DEFAULT clock_timestamp() + interval '10 minutes',
  used_at timestamptz,
  pending_pages_ciphertext bytea,
  pending_pages_nonce bytea,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CHECK ((pending_pages_ciphertext IS NULL) = (pending_pages_nonce IS NULL))
);
CREATE INDEX social_oauth_expiry ON social_oauth_states (expires_at);

CREATE TABLE social_posts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id uuid REFERENCES social_accounts(id) ON DELETE SET NULL,
  subject_kind text NOT NULL CHECK (subject_kind IN ('event', 'article')),
  event_id uuid REFERENCES event_clusters(id) ON DELETE CASCADE,
  article_id uuid REFERENCES articles(id) ON DELETE CASCADE,
  status text NOT NULL CHECK (status IN ('pending_review','approved','publishing','published','rejected','skipped','failed','needs_attention')),
  score integer NOT NULL CHECK (score BETWEEN 0 AND 100),
  selection_reason text NOT NULL DEFAULT '',
  kind text NOT NULL CHECK (kind IN ('breaking', 'top_story')),
  headline_si text NOT NULL DEFAULT '' CHECK (char_length(headline_si) <= 90),
  summary_si text NOT NULL DEFAULT '' CHECK (char_length(summary_si) <= 280),
  caption text NOT NULL DEFAULT '',
  hashtags text[] NOT NULL DEFAULT '{}' CHECK (cardinality(hashtags) <= 3),
  link_url text NOT NULL,
  source_names text[] NOT NULL DEFAULT '{}',
  source_count integer NOT NULL CHECK (source_count >= 0),
  spectrum jsonb CHECK (spectrum IS NULL OR jsonb_typeof(spectrum) = 'object'),
  image_key text,
  image_rendered_at timestamptz,
  scheduled_for timestamptz,
  approved_by uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  approved_at timestamptz,
  rejected_by uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  rejected_at timestamptz,
  published_at timestamptz,
  fb_post_id text,
  fb_permalink text,
  attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  last_error text,
  llm_call_id bigint REFERENCES llm_calls(id) ON DELETE SET NULL,
  evaluated_source_count integer NOT NULL DEFAULT 0 CHECK (evaluated_source_count >= 0),
  evaluated_breaking boolean NOT NULL DEFAULT false,
  created_by text NOT NULL DEFAULT 'selector',
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CHECK ((subject_kind = 'event' AND event_id IS NOT NULL AND article_id IS NULL)
      OR (subject_kind = 'article' AND article_id IS NOT NULL AND event_id IS NULL))
);
CREATE UNIQUE INDEX social_event_post ON social_posts (event_id)
  WHERE status NOT IN ('rejected','skipped','failed');
CREATE UNIQUE INDEX social_article_post ON social_posts (article_id)
  WHERE status NOT IN ('rejected','skipped','failed');
CREATE INDEX social_publish_queue ON social_posts (status, scheduled_for);
CREATE INDEX social_post_history ON social_posts (published_at) WHERE status = 'published';
CREATE INDEX social_event_evaluations ON social_posts (event_id, created_at DESC);
CREATE INDEX social_article_evaluations ON social_posts (article_id, created_at DESC);
CREATE TRIGGER social_posts_set_updated_at BEFORE UPDATE ON social_posts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

INSERT INTO llm_task_profiles (task, provider_id, model, reasoning_effort, timeout_seconds, enabled)
VALUES ('social_post_editorial', 'codex_cli', 'gpt-6.1-sol', 'medium', 300, true);
INSERT INTO agent_workflows (task, name, purpose, category, personality, tone, response_language, audience)
VALUES ('social_post_editorial', 'Social post editorial', 'Selects public-interest stories and writes original Sinhala Facebook Page headlines, summaries and captions from rights-approved synthesis.',
        'Social publishing', 'Careful Sinhala newsroom editor who preserves facts and source attribution.',
        'factual and neutral', 'si', 'Sri Lankan readers');
INSERT INTO agent_workflow_versions (task, revision, snapshot)
SELECT task, revision, jsonb_build_object(
  'custom_instructions', custom_instructions, 'personality', personality,
  'learning_notes', learning_notes, 'tone', tone, 'response_language', response_language,
  'audience', audience, 'enabled', enabled
) FROM agent_workflows WHERE task = 'social_post_editorial';

COMMIT;
