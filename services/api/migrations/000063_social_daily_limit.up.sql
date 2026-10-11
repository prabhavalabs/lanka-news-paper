BEGIN;
-- The daily limit is now soft: important and breaking posts pass it. 30 leaves room for a busy news day.
ALTER TABLE social_settings ALTER COLUMN max_posts_per_day SET DEFAULT 30;
UPDATE social_settings SET max_posts_per_day = 30 WHERE max_posts_per_day = 12;
COMMIT;
