BEGIN;

UPDATE llm_task_profiles
SET timeout_seconds = 300, max_output_tokens = NULL
WHERE task = 'narration_framing' AND provider_id = 'vps-ollama';

COMMIT;
