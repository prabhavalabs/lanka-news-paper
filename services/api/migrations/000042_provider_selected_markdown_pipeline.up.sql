BEGIN;

ALTER TABLE article_pipeline_runs
  ADD COLUMN provider_id text,
  ADD COLUMN provider_model text,
  ADD CONSTRAINT article_pipeline_provider_selection_complete CHECK (
    (provider_id IS NULL AND provider_model IS NULL)
    OR (length(btrim(provider_id)) > 0 AND length(btrim(provider_model)) > 0)
  );

ALTER TABLE article_analysis_documents
  ADD COLUMN cleaner_provider text NOT NULL DEFAULT 'deterministic',
  ADD COLUMN cleaner_model text NOT NULL DEFAULT 'editorial-cleaner-v3';

INSERT INTO llm_task_profiles (
  task, provider_id, model, reasoning_effort, max_output_tokens,
  temperature, timeout_seconds, enabled
)
SELECT 'content_cleaning', profile.provider_id, profile.model, profile.reasoning_effort,
       6000, profile.temperature, GREATEST(profile.timeout_seconds, 300), profile.enabled
FROM llm_task_profiles profile
WHERE profile.task = 'article_summary'
ON CONFLICT (task) DO UPDATE SET
  provider_id = EXCLUDED.provider_id,
  model = EXCLUDED.model,
  reasoning_effort = EXCLUDED.reasoning_effort,
  timeout_seconds = EXCLUDED.timeout_seconds,
  max_output_tokens = EXCLUDED.max_output_tokens,
  temperature = EXCLUDED.temperature,
  enabled = EXCLUDED.enabled;

COMMIT;
