BEGIN;

ALTER TABLE llm_calls
  ADD COLUMN streamed boolean NOT NULL DEFAULT false,
  ADD COLUMN first_token_ms integer,
  ADD COLUMN response_text text,
  ADD COLUMN finish_reason text,
  ADD COLUMN completed_at timestamptz;

CREATE TABLE article_pipeline_logs (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  run_id uuid NOT NULL REFERENCES article_pipeline_runs(id) ON DELETE CASCADE,
  step_id uuid REFERENCES article_pipeline_steps(id) ON DELETE CASCADE,
  level text NOT NULL DEFAULT 'info',
  event text NOT NULL,
  message text NOT NULL,
  details jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT article_pipeline_log_level_valid
    CHECK (level IN ('info', 'warning', 'error'))
);

CREATE INDEX article_pipeline_logs_run
  ON article_pipeline_logs (run_id, created_at, id);

CREATE INDEX article_pipeline_logs_step
  ON article_pipeline_logs (step_id, created_at, id);

UPDATE llm_task_profiles
SET timeout_seconds = 600
WHERE task = 'classify' AND provider_id = 'vps-ollama';

UPDATE llm_task_profiles
SET timeout_seconds = 900
WHERE task = 'narration_framing' AND provider_id = 'vps-ollama';

COMMIT;
