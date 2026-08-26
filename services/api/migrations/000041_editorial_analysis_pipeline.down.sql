BEGIN;

ALTER TABLE admin_analysis_backfills
  DROP CONSTRAINT IF EXISTS admin_analysis_backfill_workflow_valid,
  DROP CONSTRAINT IF EXISTS admin_analysis_backfill_provider_valid,
  DROP COLUMN IF EXISTS workflow,
  ADD CONSTRAINT admin_analysis_backfill_provider_valid
    CHECK (provider IN ('openrouter', 'codex_cli'));

DELETE FROM llm_task_profiles WHERE task IN ('article_summary', 'event_synthesis');
DROP TABLE IF EXISTS event_narrative_analyses;

ALTER TABLE article_political_analysis
  DROP CONSTRAINT IF EXISTS article_political_probability_total_valid,
  DROP CONSTRAINT IF EXISTS article_political_right_probability_valid,
  DROP CONSTRAINT IF EXISTS article_political_center_probability_valid,
  DROP CONSTRAINT IF EXISTS article_political_left_probability_valid,
  DROP COLUMN IF EXISTS axis_version,
  DROP COLUMN IF EXISTS right_probability,
  DROP COLUMN IF EXISTS center_probability,
  DROP COLUMN IF EXISTS left_probability;

DROP TABLE IF EXISTS article_analysis_documents;

COMMIT;
