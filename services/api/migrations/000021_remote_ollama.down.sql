DELETE FROM llm_task_profiles
WHERE task = 'narration_framing' AND priority = 5 AND provider_id = 'vps-ollama';

DELETE FROM llm_providers WHERE id = 'vps-ollama';
