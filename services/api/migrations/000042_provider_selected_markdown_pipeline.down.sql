BEGIN;

DELETE FROM llm_task_profiles WHERE task = 'content_cleaning';

ALTER TABLE article_analysis_documents
  DROP COLUMN IF EXISTS cleaner_model,
  DROP COLUMN IF EXISTS cleaner_provider;

ALTER TABLE article_pipeline_runs
  DROP CONSTRAINT IF EXISTS article_pipeline_provider_selection_complete,
  DROP COLUMN IF EXISTS provider_model,
  DROP COLUMN IF EXISTS provider_id;

COMMIT;
