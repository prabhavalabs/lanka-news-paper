BEGIN;

UPDATE llm_task_profiles
SET max_output_tokens = 12000,
    timeout_seconds = GREATEST(timeout_seconds, 300)
WHERE task = 'content_cleaning';

COMMIT;
