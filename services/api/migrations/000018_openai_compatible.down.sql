DELETE FROM llm_task_profiles WHERE task = 'narration_framing' AND provider_id = 'local-ollama';
DELETE FROM llm_providers WHERE id = 'local-ollama';

ALTER TABLE llm_providers DROP CONSTRAINT llm_providers_kind_valid;
ALTER TABLE llm_providers ADD CONSTRAINT llm_providers_kind_valid
  CHECK (kind IN ('codex_cli', 'openai_api'));
