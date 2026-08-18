UPDATE llm_providers
SET enabled = true, status = 'unknown'
WHERE id = 'vps-ollama';
