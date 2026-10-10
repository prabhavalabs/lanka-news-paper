BEGIN;
ALTER TABLE social_posts DROP COLUMN evidence_article_ids;
ALTER TABLE social_accounts DROP COLUMN publish_retry_at;
COMMIT;
