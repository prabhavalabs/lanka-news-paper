BEGIN;
UPDATE llm_task_profiles SET reasoning_effort = 'medium', timeout_seconds = 300 WHERE task = 'social_post_review';
COMMIT;
