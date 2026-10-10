BEGIN;

INSERT INTO llm_task_profiles (task, provider_id, model, reasoning_effort, timeout_seconds, enabled)
VALUES ('article_analysis', 'codex_cli', 'gpt-6-luna', 'medium', 240, true);

INSERT INTO agent_workflows (task, name, purpose, category, personality, tone, response_language, audience)
VALUES ('article_analysis', 'Article analysis', 'Cleans, summarizes, categorizes and evaluates narration for one article in a single call.',
        'Editorial pipeline', 'Careful newsroom editor who preserves facts, attribution, and the source language.',
        'analytical and neutral', 'source', 'newsroom editors');

INSERT INTO agent_workflow_versions (task, revision, snapshot)
SELECT task, revision, jsonb_build_object(
  'custom_instructions', custom_instructions, 'personality', personality,
  'learning_notes', learning_notes, 'tone', tone, 'response_language', response_language,
  'audience', audience, 'enabled', enabled
) FROM agent_workflows WHERE task = 'article_analysis';

ALTER TABLE article_pipeline_runs
  ADD COLUMN combined_analysis jsonb,
  ADD COLUMN combined_analysis_call_id bigint REFERENCES llm_calls(id) ON DELETE SET NULL,
  ADD CONSTRAINT article_pipeline_combined_analysis_object
    CHECK (combined_analysis IS NULL OR jsonb_typeof(combined_analysis) = 'object');

COMMIT;
