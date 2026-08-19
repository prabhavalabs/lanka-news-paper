UPDATE llm_task_profiles
SET max_output_tokens = 320
WHERE task = 'narration_framing' AND provider_id = 'openrouter';
