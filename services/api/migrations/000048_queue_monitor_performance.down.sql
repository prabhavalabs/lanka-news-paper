BEGIN;

DROP INDEX IF EXISTS river_job_kind_created_at_id;
DROP INDEX IF EXISTS river_job_article_pipeline_run;
DROP INDEX IF EXISTS article_pipeline_runs_created_at_id;

COMMIT;
