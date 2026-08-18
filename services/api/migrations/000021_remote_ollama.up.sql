INSERT INTO llm_providers (id, kind, base_url, api_key_ref, enabled, status)
VALUES (
  'vps-ollama',
  'openai_compatible',
  'https://llm.lankanewspaper.prabhavalabs.com/v1',
  'SNAP_LLM_API_KEY',
  false,
  'unknown'
)
ON CONFLICT (id) DO UPDATE SET
  kind = EXCLUDED.kind,
  base_url = EXCLUDED.base_url,
  api_key_ref = EXCLUDED.api_key_ref;

INSERT INTO llm_task_profiles (task, priority, provider_id, model, timeout_seconds, enabled)
VALUES ('narration_framing', 5, 'vps-ollama', 'qwen3:8b-q4_K_M', 300, true)
ON CONFLICT (task, priority) DO UPDATE SET
  provider_id = EXCLUDED.provider_id,
  model = EXCLUDED.model,
  timeout_seconds = EXCLUDED.timeout_seconds,
  enabled = EXCLUDED.enabled;
