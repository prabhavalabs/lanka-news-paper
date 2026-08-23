BEGIN;

DROP INDEX IF EXISTS article_contents_retention;
CREATE INDEX article_contents_retention
  ON article_contents (retention_until, id)
  WHERE retention_until IS NOT NULL;

COMMIT;
