-- Deleting expired pipeline history sets these references to NULL; without indexes each
-- deleted row scanned the whole referencing table and the nightly cleanup timed out.
CREATE INDEX IF NOT EXISTS llm_calls_pipeline_run ON llm_calls (pipeline_run_id) WHERE pipeline_run_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS llm_calls_pipeline_step ON llm_calls (pipeline_step_id) WHERE pipeline_step_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS article_pipeline_runs_combined_call ON article_pipeline_runs (combined_analysis_call_id) WHERE combined_analysis_call_id IS NOT NULL;
