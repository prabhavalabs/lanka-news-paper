BEGIN;

UPDATE llm_task_profiles
SET provider_id = 'vps-ollama', model = 'qwen3:8b-q4_K_M', timeout_seconds = 600,
    max_output_tokens = 64, enabled = true
WHERE task = 'classify' AND priority = 5;

UPDATE llm_task_profiles
SET provider_id = 'vps-ollama', model = 'qwen3:8b-q4_K_M', timeout_seconds = 900,
    max_output_tokens = 320, enabled = true
WHERE task = 'narration_framing' AND priority = 5;

UPDATE llm_providers SET enabled = true WHERE id = 'vps-ollama';
DELETE FROM llm_providers WHERE id = 'openrouter';

COMMIT;
