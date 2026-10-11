BEGIN;
-- The social review now decides alone whether a post goes out, so it thinks harder.
UPDATE llm_task_profiles SET reasoning_effort = 'high', timeout_seconds = GREATEST(timeout_seconds, 420)
WHERE task = 'social_post_review';
COMMIT;
