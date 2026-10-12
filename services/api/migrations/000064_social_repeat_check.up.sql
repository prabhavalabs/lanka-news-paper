BEGIN;
-- When the publisher last confirmed a queued automatic post does not repeat a published one.
ALTER TABLE social_posts ADD COLUMN repeat_checked_at timestamptz;
COMMIT;
