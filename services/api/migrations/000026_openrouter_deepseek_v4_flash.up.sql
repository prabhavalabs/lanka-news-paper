BEGIN;

INSERT INTO llm_providers (id, kind, base_url, api_key_ref, enabled, status)
VALUES (
  'openrouter',
  'openai_api',
  'https://openrouter.ai/api/v1',
  'OPENROUTER_API_KEY',
  true,
  'unknown'
)
ON CONFLICT (id) DO UPDATE SET
  kind = EXCLUDED.kind,
  base_url = EXCLUDED.base_url,
  api_key_ref = EXCLUDED.api_key_ref,
  enabled = EXCLUDED.enabled,
  status = EXCLUDED.status;

UPDATE llm_providers
SET enabled = false
WHERE id IN ('local-ollama', 'vps-ollama');

INSERT INTO llm_task_profiles (
  task, priority, provider_id, model, timeout_seconds, max_output_tokens, enabled
)
VALUES
  ('classify', 5, 'openrouter', 'deepseek/deepseek-v4-flash-0731', 120, 64, true),
  ('narration_framing', 5, 'openrouter', 'deepseek/deepseek-v4-flash-0731', 120, 1024, true)
ON CONFLICT (task, priority) DO UPDATE SET
  provider_id = EXCLUDED.provider_id,
  model = EXCLUDED.model,
  timeout_seconds = EXCLUDED.timeout_seconds,
  max_output_tokens = EXCLUDED.max_output_tokens,
  enabled = EXCLUDED.enabled;

UPDATE llm_task_profiles
SET enabled = false
WHERE provider_id IN ('local-ollama', 'vps-ollama');

COMMIT;
