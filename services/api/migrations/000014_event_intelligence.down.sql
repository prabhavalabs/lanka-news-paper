BEGIN;

DROP INDEX IF EXISTS articles_unclustered_published;
ALTER TABLE event_articles DROP COLUMN IF EXISTS signals;
ALTER TABLE event_articles DROP COLUMN IF EXISTS decided_at;
ALTER TABLE event_clusters DROP COLUMN IF EXISTS algorithm_version;

COMMIT;
