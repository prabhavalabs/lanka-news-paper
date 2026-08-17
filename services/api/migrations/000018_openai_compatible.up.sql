ALTER TABLE llm_providers DROP CONSTRAINT llm_providers_kind_valid;
ALTER TABLE llm_providers ADD CONSTRAINT llm_providers_kind_valid
  CHECK (kind IN ('codex_cli', 'openai_api', 'openai_compatible'));

INSERT INTO llm_providers (id, kind, base_url, enabled, status)
VALUES ('local-ollama', 'openai_compatible', 'http://host.docker.internal:11434/v1', false, 'unknown')
ON CONFLICT (id) DO NOTHING;

INSERT INTO llm_task_profiles (task, priority, provider_id, model, timeout_seconds, enabled)
VALUES ('narration_framing', 10, 'local-ollama', 'qwen3:4b', 90, true)
ON CONFLICT (task, priority) DO NOTHING;
