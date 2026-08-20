BEGIN;

ALTER TABLE llm_task_profiles DROP CONSTRAINT llm_task_profiles_pkey;
ALTER TABLE llm_task_profiles ADD COLUMN priority integer NOT NULL DEFAULT 5;
ALTER TABLE llm_task_profiles ADD PRIMARY KEY (task, priority);

ALTER TABLE llm_providers DROP CONSTRAINT llm_providers_kind_valid;
ALTER TABLE llm_providers ADD CONSTRAINT llm_providers_kind_valid
  CHECK (kind IN ('codex_cli', 'openai_api', 'openai_compatible'));

COMMIT;
