BEGIN;

DROP INDEX IF EXISTS article_contents_retention;
CREATE INDEX article_contents_retention
  ON article_contents (retention_until)
  WHERE current AND retention_until IS NOT NULL;

COMMIT;
