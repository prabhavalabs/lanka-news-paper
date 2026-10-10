ALTER TABLE social_posts DROP COLUMN IF EXISTS review_revisions;
DROP INDEX IF EXISTS social_posts_recent;
ALTER TABLE social_posts DROP COLUMN IF EXISTS hold_reason;
