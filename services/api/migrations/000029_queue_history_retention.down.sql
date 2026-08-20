BEGIN;

DROP INDEX IF EXISTS article_pipeline_runs_finished;
DROP TRIGGER IF EXISTS article_pipeline_mark_enqueued ON article_pipeline_runs;
DROP FUNCTION IF EXISTS mark_article_pipeline_enqueued();
ALTER TABLE articles DROP COLUMN IF EXISTS pipeline_enqueued_at;

COMMIT;
