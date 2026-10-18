BEGIN;
-- Facebook posts now go out through Loomcast's machine API. The Meta app's Page token and its
-- OAuth state are retired; Loomcast holds the Page connection.
ALTER TABLE social_posts
  DROP COLUMN account_id,
  ADD COLUMN loomcast_post_id text,
  ADD COLUMN handed_off_at timestamptz;
CREATE UNIQUE INDEX social_posts_loomcast_post ON social_posts (loomcast_post_id) WHERE loomcast_post_id IS NOT NULL;
-- A refusal from Loomcast (rate limit or an API key problem) pauses every post, not just one.
ALTER TABLE social_settings ADD COLUMN publish_retry_at timestamptz;
DROP TABLE social_oauth_states;
DROP TABLE social_accounts;
COMMIT;
