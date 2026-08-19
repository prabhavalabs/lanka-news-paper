BEGIN;

UPDATE llm_task_profiles
SET timeout_seconds = 300
WHERE task = 'classify' AND provider_id = 'vps-ollama';

UPDATE llm_task_profiles
SET timeout_seconds = 600
WHERE task = 'narration_framing' AND provider_id = 'vps-ollama';

DROP TABLE article_pipeline_logs;

ALTER TABLE llm_calls
  DROP COLUMN completed_at,
  DROP COLUMN finish_reason,
  DROP COLUMN response_text,
  DROP COLUMN first_token_ms,
  DROP COLUMN streamed;

COMMIT;
