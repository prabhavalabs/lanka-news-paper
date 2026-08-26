BEGIN;

CREATE TABLE admin_analysis_backfills (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  scope text NOT NULL,
  provider text NOT NULL,
  model text NOT NULL,
  from_at timestamptz,
  to_at timestamptz,
  article_id uuid REFERENCES articles(id) ON DELETE SET NULL,
  status text NOT NULL DEFAULT 'queued',
  total_articles integer NOT NULL DEFAULT 0,
  created_by text NOT NULL,
  error_detail text,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  started_at timestamptz,
  finished_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT admin_analysis_backfill_scope_valid
    CHECK (scope IN ('date_range', 'catalog', 'article')),
  CONSTRAINT admin_analysis_backfill_provider_valid
    CHECK (provider IN ('openrouter', 'codex_cli')),
  CONSTRAINT admin_analysis_backfill_status_valid
    CHECK (status IN ('queued', 'running', 'completed', 'partially_completed', 'failed')),
  CONSTRAINT admin_analysis_backfill_total_nonnegative CHECK (total_articles >= 0)
);

CREATE INDEX admin_analysis_backfills_recent
  ON admin_analysis_backfills (created_at DESC, id DESC);

CREATE TABLE admin_analysis_backfill_items (
  run_id uuid NOT NULL REFERENCES admin_analysis_backfills(id) ON DELETE CASCADE,
  article_id uuid NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  state text NOT NULL DEFAULT 'pending',
  river_job_id bigint,
  attempt integer NOT NULL DEFAULT 0,
  error_detail text,
  queued_at timestamptz,
  started_at timestamptz,
  finished_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY (run_id, article_id),
  CONSTRAINT admin_analysis_backfill_item_state_valid
    CHECK (state IN ('pending', 'queued', 'running', 'succeeded', 'failed')),
  CONSTRAINT admin_analysis_backfill_item_attempt_nonnegative CHECK (attempt >= 0)
);

CREATE INDEX admin_analysis_backfill_items_dispatch
  ON admin_analysis_backfill_items (run_id, article_id)
  WHERE state = 'pending';

CREATE INDEX admin_analysis_backfill_items_progress
  ON admin_analysis_backfill_items (run_id, state);

CREATE TABLE article_ai_insights (
  article_id uuid PRIMARY KEY REFERENCES articles(id) ON DELETE CASCADE,
  backfill_run_id uuid REFERENCES admin_analysis_backfills(id) ON DELETE SET NULL,
  summary text NOT NULL,
  tone text NOT NULL,
  political_relevance boolean NOT NULL,
  political_narrative text NOT NULL DEFAULT '',
  spectrum_score numeric NOT NULL,
  confidence numeric NOT NULL,
  evidence jsonb NOT NULL DEFAULT '[]'::jsonb,
  provider text NOT NULL,
  provider_model text NOT NULL,
  schema_version text NOT NULL DEFAULT 'article-insight-v1',
  analyzed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT article_ai_insights_tone_valid
    CHECK (tone IN ('neutral', 'positive', 'negative', 'mixed', 'urgent')),
  CONSTRAINT article_ai_insights_spectrum_valid CHECK (spectrum_score BETWEEN -1 AND 1),
  CONSTRAINT article_ai_insights_confidence_valid CHECK (confidence BETWEEN 0 AND 1),
  CONSTRAINT article_ai_insights_evidence_array CHECK (jsonb_typeof(evidence) = 'array'),
  CONSTRAINT article_ai_insights_provider_valid CHECK (provider IN ('openrouter', 'codex_cli'))
);

CREATE INDEX article_ai_insights_analyzed_at
  ON article_ai_insights (analyzed_at DESC, article_id);

COMMIT;
