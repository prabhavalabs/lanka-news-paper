BEGIN;
ALTER TABLE social_settings ALTER COLUMN max_posts_per_day SET DEFAULT 12;
UPDATE social_settings SET max_posts_per_day = 12 WHERE max_posts_per_day = 30;
COMMIT;
