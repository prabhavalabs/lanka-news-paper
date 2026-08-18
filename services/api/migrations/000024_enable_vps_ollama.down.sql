UPDATE llm_providers
SET enabled = false, status = 'unknown'
WHERE id = 'vps-ollama';
