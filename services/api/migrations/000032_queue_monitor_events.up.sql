BEGIN;

CREATE FUNCTION notify_queue_monitor_change()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM pg_notify('queue_monitor_changed', TG_TABLE_NAME);
  RETURN NULL;
END;
$$;

CREATE TRIGGER queue_monitor_pipeline_runs_changed
AFTER INSERT OR UPDATE OR DELETE ON article_pipeline_runs
FOR EACH STATEMENT EXECUTE FUNCTION notify_queue_monitor_change();

CREATE TRIGGER queue_monitor_pipeline_steps_changed
AFTER INSERT OR UPDATE OR DELETE ON article_pipeline_steps
FOR EACH STATEMENT EXECUTE FUNCTION notify_queue_monitor_change();

CREATE TRIGGER queue_monitor_river_jobs_changed
AFTER INSERT OR UPDATE OR DELETE ON river_job
FOR EACH STATEMENT EXECUTE FUNCTION notify_queue_monitor_change();

CREATE TRIGGER queue_monitor_river_queues_changed
AFTER INSERT OR UPDATE OR DELETE ON river_queue
FOR EACH STATEMENT EXECUTE FUNCTION notify_queue_monitor_change();

COMMIT;
