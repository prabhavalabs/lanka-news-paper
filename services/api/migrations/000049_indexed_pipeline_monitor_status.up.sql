BEGIN;

ALTER TABLE article_pipeline_runs
  ADD COLUMN monitor_status text NOT NULL DEFAULT 'queued',
  ADD CONSTRAINT article_pipeline_monitor_status_valid
    CHECK (monitor_status IN ('queued', 'processing', 'completed', 'partially_completed', 'failed'));

WITH step_state AS (
  SELECT run.id,
         count(step.id)::integer AS total,
         count(*) FILTER (WHERE step.status = 'running')::integer AS running,
         count(*) FILTER (WHERE step.status = 'failed')::integer AS failed,
         count(*) FILTER (WHERE step.status = 'succeeded')::integer AS succeeded,
         count(*) FILTER (WHERE step.status IN ('succeeded', 'skipped'))::integer AS terminal
  FROM article_pipeline_runs run
  LEFT JOIN article_pipeline_steps step ON step.run_id = run.id
  GROUP BY run.id
)
UPDATE article_pipeline_runs run
SET monitor_status = CASE
  WHEN run.status = 'running' OR state.running > 0 THEN 'processing'
  WHEN state.failed > 0 AND state.succeeded > 0 THEN 'partially_completed'
  WHEN state.failed > 0 THEN 'failed'
  WHEN state.total > 0 AND state.terminal = state.total THEN 'completed'
  WHEN run.status = 'succeeded' THEN 'completed'
  WHEN run.status = 'failed' THEN 'failed'
  ELSE 'queued'
END
FROM step_state state
WHERE state.id = run.id;

CREATE OR REPLACE FUNCTION article_pipeline_monitor_status(
  selected_run_id uuid,
  selected_run_status text
) RETURNS text
LANGUAGE sql
STABLE
AS $$
  SELECT CASE
    WHEN selected_run_status = 'running'
      OR count(*) FILTER (WHERE step.status = 'running') > 0
      THEN 'processing'
    WHEN count(*) FILTER (WHERE step.status = 'failed') > 0
      AND count(*) FILTER (WHERE step.status = 'succeeded') > 0
      THEN 'partially_completed'
    WHEN count(*) FILTER (WHERE step.status = 'failed') > 0
      THEN 'failed'
    WHEN count(*) > 0
      AND count(*) FILTER (WHERE step.status IN ('succeeded', 'skipped')) = count(*)
      THEN 'completed'
    WHEN selected_run_status = 'succeeded' THEN 'completed'
    WHEN selected_run_status = 'failed' THEN 'failed'
    ELSE 'queued'
  END
  FROM article_pipeline_steps step
  WHERE step.run_id = selected_run_id;
$$;

CREATE OR REPLACE FUNCTION set_article_pipeline_run_monitor_status()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.monitor_status := article_pipeline_monitor_status(NEW.id, NEW.status);
  RETURN NEW;
END;
$$;

CREATE TRIGGER article_pipeline_runs_set_monitor_status
BEFORE INSERT OR UPDATE OF status ON article_pipeline_runs
FOR EACH ROW EXECUTE FUNCTION set_article_pipeline_run_monitor_status();

CREATE OR REPLACE FUNCTION refresh_article_pipeline_step_monitor_status()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  affected_run_id uuid := COALESCE(NEW.run_id, OLD.run_id);
BEGIN
  UPDATE article_pipeline_runs run
  SET monitor_status = article_pipeline_monitor_status(run.id, run.status)
  WHERE run.id = affected_run_id;
  RETURN COALESCE(NEW, OLD);
END;
$$;

CREATE TRIGGER article_pipeline_steps_refresh_monitor_status
AFTER INSERT OR UPDATE OF status OR DELETE ON article_pipeline_steps
FOR EACH ROW EXECUTE FUNCTION refresh_article_pipeline_step_monitor_status();

CREATE INDEX article_pipeline_runs_monitor_created_id
  ON article_pipeline_runs (monitor_status, created_at DESC, id DESC);

COMMIT;
