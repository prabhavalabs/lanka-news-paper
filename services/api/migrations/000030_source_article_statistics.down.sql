BEGIN;

DROP TRIGGER IF EXISTS articles_maintain_source_article_statistics ON articles;
DROP FUNCTION IF EXISTS maintain_source_article_statistics();
DROP FUNCTION IF EXISTS remove_published_article_stat(uuid, timestamptz);
DROP FUNCTION IF EXISTS add_published_article_stat(uuid, timestamptz);
DROP INDEX IF EXISTS articles_published_by_source;
DROP TABLE IF EXISTS source_article_statistics;

COMMIT;
