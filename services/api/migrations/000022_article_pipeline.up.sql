BEGIN;

CREATE TABLE article_pipeline_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  article_id uuid NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  status text NOT NULL DEFAULT 'queued',
  trigger text NOT NULL DEFAULT 'ingestion',
  current_step text,
  attempt integer NOT NULL DEFAULT 0,
  last_error text,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  started_at timestamptz,
  finished_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT article_pipeline_run_status_valid
    CHECK (status IN ('queued', 'running', 'succeeded', 'failed'))
);

CREATE UNIQUE INDEX article_pipeline_one_active_run
  ON article_pipeline_runs (article_id)
  WHERE status IN ('queued', 'running');

CREATE INDEX article_pipeline_runs_article
  ON article_pipeline_runs (article_id, created_at DESC);

CREATE TABLE article_pipeline_steps (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id uuid NOT NULL REFERENCES article_pipeline_runs(id) ON DELETE CASCADE,
  name text NOT NULL,
  position integer NOT NULL,
  status text NOT NULL DEFAULT 'queued',
  attempt integer NOT NULL DEFAULT 0,
  max_attempts integer NOT NULL DEFAULT 5,
  started_at timestamptz,
  finished_at timestamptz,
  duration_ms bigint,
  error_detail text,
  output jsonb NOT NULL DEFAULT '{}'::jsonb,
  CONSTRAINT article_pipeline_step_status_valid
    CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'skipped')),
  UNIQUE (run_id, name),
  UNIQUE (run_id, position)
);

ALTER TABLE llm_calls
  ADD COLUMN article_id uuid REFERENCES articles(id) ON DELETE SET NULL,
  ADD COLUMN pipeline_run_id uuid REFERENCES article_pipeline_runs(id) ON DELETE SET NULL,
  ADD COLUMN pipeline_step_id uuid REFERENCES article_pipeline_steps(id) ON DELETE SET NULL,
  ADD COLUMN error_detail text;

CREATE INDEX llm_calls_article ON llm_calls (article_id, created_at DESC);

INSERT INTO llm_task_profiles (task, priority, provider_id, model, timeout_seconds, max_output_tokens, enabled)
VALUES ('classify', 5, 'vps-ollama', 'qwen3:8b-q4_K_M', 300, 64, true)
ON CONFLICT (task, priority) DO UPDATE SET
  provider_id = EXCLUDED.provider_id,
  model = EXCLUDED.model,
  timeout_seconds = EXCLUDED.timeout_seconds,
  max_output_tokens = EXCLUDED.max_output_tokens,
  enabled = EXCLUDED.enabled;

COMMIT;
