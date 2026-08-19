UPDATE llm_task_profiles
SET max_output_tokens = 1024
WHERE task = 'narration_framing' AND provider_id = 'openrouter';
