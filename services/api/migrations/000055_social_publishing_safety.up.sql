BEGIN;
ALTER TABLE social_accounts ADD COLUMN publish_retry_at timestamptz;
ALTER TABLE social_posts ADD COLUMN evidence_article_ids uuid[] NOT NULL DEFAULT '{}';
COMMIT;
