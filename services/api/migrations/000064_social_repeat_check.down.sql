BEGIN;
ALTER TABLE social_posts DROP COLUMN repeat_checked_at;
COMMIT;
