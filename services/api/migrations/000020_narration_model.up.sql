UPDATE llm_task_profiles
SET model = 'qwen3:8b'
WHERE task = 'narration_framing'
  AND provider_id = 'local-ollama';
