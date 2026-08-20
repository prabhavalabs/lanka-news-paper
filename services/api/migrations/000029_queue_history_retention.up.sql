BEGIN;

ALTER TABLE articles ADD COLUMN pipeline_enqueued_at timestamptz;

UPDATE articles article
SET pipeline_enqueued_at = history.first_enqueued_at
FROM (
  SELECT article_id, min(created_at) AS first_enqueued_at
  FROM article_pipeline_runs
  GROUP BY article_id
) history
WHERE article.id = history.article_id;

CREATE FUNCTION mark_article_pipeline_enqueued()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  UPDATE articles
  SET pipeline_enqueued_at = COALESCE(pipeline_enqueued_at, NEW.created_at)
  WHERE id = NEW.article_id;
  RETURN NEW;
END;
$$;

CREATE TRIGGER article_pipeline_mark_enqueued
AFTER INSERT ON article_pipeline_runs
FOR EACH ROW EXECUTE FUNCTION mark_article_pipeline_enqueued();

CREATE INDEX article_pipeline_runs_finished
  ON article_pipeline_runs (finished_at)
  WHERE status IN ('succeeded', 'failed');

DELETE FROM llm_calls
WHERE pipeline_run_id IN (
  SELECT id FROM article_pipeline_runs
  WHERE status IN ('succeeded', 'failed')
    AND finished_at < clock_timestamp() - interval '7 days'
);

DELETE FROM article_pipeline_runs
WHERE status IN ('succeeded', 'failed')
  AND finished_at < clock_timestamp() - interval '7 days';

COMMIT;
