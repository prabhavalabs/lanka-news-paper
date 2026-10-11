CREATE TABLE log_retention_settings (
  singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
  retention_days integer NOT NULL DEFAULT 7 CHECK (retention_days BETWEEN 1 AND 365),
  last_run_at timestamptz,
  last_run_summary jsonb NOT NULL DEFAULT '{}'::jsonb,
  last_run_error text,
  updated_by uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
INSERT INTO log_retention_settings (singleton) VALUES (true);

-- The cleanup deletes by age; these indexes keep each batch from scanning whole tables.
CREATE INDEX IF NOT EXISTS crawl_attempts_started ON crawl_attempts (started_at);
CREATE INDEX IF NOT EXISTS ingestion_runs_started ON ingestion_runs (started_at);
CREATE INDEX IF NOT EXISTS article_pipeline_logs_created ON article_pipeline_logs (created_at);
CREATE INDEX IF NOT EXISTS llm_calls_created ON llm_calls (created_at);
