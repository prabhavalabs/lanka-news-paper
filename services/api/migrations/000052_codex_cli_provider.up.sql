BEGIN;

-- reasoning_effort already exists in the original operations schema.
ALTER TABLE llm_task_profiles ADD COLUMN IF NOT EXISTS reasoning_effort text;
ALTER TABLE llm_providers DROP CONSTRAINT llm_providers_kind_valid;

INSERT INTO llm_providers (id, kind, base_url, api_key_ref, enabled, status)
VALUES ('codex_cli', 'codex_cli', NULL, NULL, true, 'unknown');

UPDATE llm_task_profiles
SET provider_id = 'codex_cli', model = 'gpt-6-luna', reasoning_effort = 'medium',
    timeout_seconds = 180;

INSERT INTO llm_task_profiles (task, provider_id, model, reasoning_effort, timeout_seconds, enabled)
VALUES
  ('content_cleaning', 'codex_cli', 'gpt-6-luna', 'low', 180, true),
  ('article_summary', 'codex_cli', 'gpt-6-luna', 'medium', 180, true),
  ('classify', 'codex_cli', 'gpt-6-luna', 'low', 180, true),
  ('narration_framing', 'codex_cli', 'gpt-6-luna', 'medium', 180, true),
  ('event_synthesis', 'codex_cli', 'gpt-6-luna', 'medium', 180, true),
  ('watch_tower_retrieval', 'codex_cli', 'gpt-6-luna', 'low', 180, true),
  ('newsletter_editorial', 'codex_cli', 'gpt-6.1-sol', 'medium', 300, true),
  ('watch_tower_answer', 'codex_cli', 'gpt-6.1-sol', 'medium', 300, true),
  ('admin_article_analysis', 'codex_cli', 'gpt-6.1-sol', 'medium', 300, true)
ON CONFLICT (task) DO UPDATE SET
  provider_id = EXCLUDED.provider_id, model = EXCLUDED.model,
  reasoning_effort = EXCLUDED.reasoning_effort, timeout_seconds = EXCLUDED.timeout_seconds;

ALTER TABLE llm_task_profiles ADD CONSTRAINT llm_task_profiles_reasoning_effort_valid
  CHECK (reasoning_effort IS NULL OR reasoning_effort IN ('low', 'medium', 'high'));

-- Only task profiles reference llm_providers by FK. Call/run/insight provider
-- strings and their historical CHECK constraints are intentionally unchanged.
DELETE FROM llm_providers WHERE id <> 'codex_cli';
ALTER TABLE llm_providers ADD CONSTRAINT llm_providers_kind_valid CHECK (kind = 'codex_cli');

COMMIT;
