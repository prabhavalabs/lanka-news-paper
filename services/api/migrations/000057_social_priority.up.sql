-- Why a post that passed the AI review was held for an editor instead of publishing automatically
-- (for example, too many posts on the same topic recently), and a lookup for recent-post checks.
ALTER TABLE social_posts ADD COLUMN hold_reason text;
CREATE INDEX social_posts_recent ON social_posts (created_at DESC) WHERE status IN ('pending_review','approved','publishing','published');

-- How many times the AI rewrote a draft to fix review issues before the final review.
ALTER TABLE social_posts ADD COLUMN review_revisions integer NOT NULL DEFAULT 0 CHECK (review_revisions BETWEEN 0 AND 3);

-- Captions no longer carry the story URL; remove it from drafts that have not been published.
UPDATE social_posts SET caption = replace(caption, E'\n\n' || link_url, '')
WHERE link_url <> '' AND status IN ('pending_review','approved') AND position(link_url IN caption) > 0;
