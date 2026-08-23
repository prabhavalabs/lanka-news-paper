BEGIN;

DROP INDEX IF EXISTS river_article_content_active;
DROP INDEX IF EXISTS crawl_attempts_article_profile_failures;
DROP INDEX IF EXISTS articles_content_backfill;

COMMIT;
