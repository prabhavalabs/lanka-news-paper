-- Why a post that passed the AI review was held for an editor instead of publishing automatically
-- (for example, too many posts on the same topic recently), and a lookup for recent-post checks.
ALTER TABLE social_posts ADD COLUMN hold_reason text;
CREATE INDEX social_posts_recent ON social_posts (created_at DESC) WHERE status IN ('pending_review','approved','publishing','published');
