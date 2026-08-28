BEGIN;

CREATE TABLE newsletter_test_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  mode text NOT NULL CHECK (mode IN ('preview', 'send')),
  window_mode text NOT NULL CHECK (window_mode IN ('latest_24h', 'scheduled')),
  status text NOT NULL CHECK (status IN ('succeeded', 'failed')),
  recipient_email citext,
  provider_id text NOT NULL DEFAULT '',
  model text NOT NULL DEFAULT '',
  subject text NOT NULL DEFAULT '',
  preheader text NOT NULL DEFAULT '',
  story_count integer NOT NULL DEFAULT 0 CHECK (story_count >= 0),
  article_count integer NOT NULL DEFAULT 0 CHECK (article_count >= 0),
  event_count integer NOT NULL DEFAULT 0 CHECK (event_count >= 0),
  source_count integer NOT NULL DEFAULT 0 CHECK (source_count >= 0),
  duration_ms integer NOT NULL DEFAULT 0 CHECK (duration_ms >= 0),
  provider_message_id text NOT NULL DEFAULT '',
  error_detail text NOT NULL DEFAULT '',
  created_by uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  completed_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX newsletter_test_runs_created_at_idx
  ON newsletter_test_runs (created_at DESC, id DESC);

COMMIT;
