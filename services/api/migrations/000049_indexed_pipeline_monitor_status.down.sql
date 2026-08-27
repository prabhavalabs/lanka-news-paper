BEGIN;

DROP INDEX IF EXISTS article_pipeline_runs_monitor_created_id;
DROP TRIGGER IF EXISTS article_pipeline_steps_refresh_monitor_status ON article_pipeline_steps;
DROP FUNCTION IF EXISTS refresh_article_pipeline_step_monitor_status();
DROP TRIGGER IF EXISTS article_pipeline_runs_set_monitor_status ON article_pipeline_runs;
DROP FUNCTION IF EXISTS set_article_pipeline_run_monitor_status();
DROP FUNCTION IF EXISTS article_pipeline_monitor_status(uuid, text);
ALTER TABLE article_pipeline_runs DROP COLUMN IF EXISTS monitor_status;

COMMIT;
