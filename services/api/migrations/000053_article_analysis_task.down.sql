BEGIN;

ALTER TABLE article_pipeline_runs
  DROP CONSTRAINT article_pipeline_combined_analysis_object,
  DROP COLUMN combined_analysis_call_id,
  DROP COLUMN combined_analysis;
DELETE FROM agent_workflows WHERE task = 'article_analysis';
DELETE FROM llm_task_profiles WHERE task = 'article_analysis';

COMMIT;
