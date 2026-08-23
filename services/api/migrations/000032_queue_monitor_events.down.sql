BEGIN;

DROP TRIGGER IF EXISTS queue_monitor_river_queues_changed ON river_queue;
DROP TRIGGER IF EXISTS queue_monitor_river_jobs_changed ON river_job;
DROP TRIGGER IF EXISTS queue_monitor_pipeline_steps_changed ON article_pipeline_steps;
DROP TRIGGER IF EXISTS queue_monitor_pipeline_runs_changed ON article_pipeline_runs;
DROP FUNCTION IF EXISTS notify_queue_monitor_change();

COMMIT;
