BEGIN;

UPDATE llm_task_profiles
SET timeout_seconds = 600, max_output_tokens = 320
WHERE task = 'narration_framing' AND provider_id = 'vps-ollama';

COMMIT;
