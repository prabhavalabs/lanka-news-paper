BEGIN;

DELETE FROM llm_task_profiles
WHERE task NOT IN ('classify', 'narration_framing') OR provider_id <> 'openrouter';

DELETE FROM llm_task_profiles duplicate
USING llm_task_profiles canonical
WHERE duplicate.task = canonical.task
  AND duplicate.priority > canonical.priority;

ALTER TABLE llm_task_profiles DROP CONSTRAINT llm_task_profiles_pkey;
ALTER TABLE llm_task_profiles DROP COLUMN priority;
ALTER TABLE llm_task_profiles ADD PRIMARY KEY (task);

DELETE FROM llm_providers WHERE id <> 'openrouter';

ALTER TABLE llm_providers DROP CONSTRAINT llm_providers_kind_valid;
ALTER TABLE llm_providers ADD CONSTRAINT llm_providers_kind_valid
  CHECK (kind = 'openai_api');

UPDATE llm_providers
SET kind = 'openai_api',
    base_url = 'https://openrouter.ai/api/v1',
    api_key_ref = 'OPENROUTER_API_KEY',
    enabled = true
WHERE id = 'openrouter';

COMMIT;
