BEGIN;

ALTER TABLE event_clusters
  ADD COLUMN IF NOT EXISTS algorithm_version text NOT NULL DEFAULT 'headline-trgm-v1';

ALTER TABLE event_articles
  ADD COLUMN IF NOT EXISTS decided_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  ADD COLUMN IF NOT EXISTS signals jsonb NOT NULL DEFAULT '{}'::jsonb;

UPDATE event_articles
SET clustering_score = 0.5
WHERE clustering_score IS NULL;

CREATE INDEX IF NOT EXISTS articles_unclustered_published
  ON articles (published_at, id)
  WHERE public_status = 'published' AND event_id IS NULL;

COMMIT;
