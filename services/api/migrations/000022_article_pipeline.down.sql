BEGIN;

DELETE FROM llm_task_profiles WHERE task = 'classify' AND priority = 5 AND provider_id = 'vps-ollama';
ALTER TABLE llm_calls
  DROP COLUMN IF EXISTS error_detail,
  DROP COLUMN IF EXISTS pipeline_step_id,
  DROP COLUMN IF EXISTS pipeline_run_id,
  DROP COLUMN IF EXISTS article_id;
DROP TABLE IF EXISTS article_pipeline_steps;
DROP TABLE IF EXISTS article_pipeline_runs;

COMMIT;
