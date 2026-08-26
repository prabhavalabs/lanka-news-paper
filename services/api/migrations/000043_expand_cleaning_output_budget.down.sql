BEGIN;

UPDATE llm_task_profiles
SET max_output_tokens = 6000
WHERE task = 'content_cleaning';

COMMIT;
