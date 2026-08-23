BEGIN;

CREATE INDEX articles_content_backfill
  ON articles (endpoint_id, received_at, id);

CREATE INDEX crawl_attempts_article_profile_failures
  ON crawl_attempts (article_id, collection_profile_id)
  WHERE status IN ('failed', 'blocked');

CREATE INDEX river_article_content_active
  ON river_job ((args->>'article_id'))
  WHERE kind = 'article.content'
    AND state IN ('available', 'pending', 'retryable', 'running', 'scheduled');

COMMIT;
