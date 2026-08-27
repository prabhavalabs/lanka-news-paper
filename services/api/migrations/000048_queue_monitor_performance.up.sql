BEGIN;

CREATE INDEX article_pipeline_runs_created_at_id
  ON article_pipeline_runs (created_at DESC, id DESC);

CREATE INDEX river_job_article_pipeline_run
  ON river_job ((args->>'run_id'), created_at DESC, id DESC)
  WHERE kind = 'article.pipeline';

CREATE INDEX river_job_kind_created_at_id
  ON river_job (kind, created_at DESC, id DESC);

COMMIT;
