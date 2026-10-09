BEGIN;

ALTER TABLE llm_providers DROP CONSTRAINT llm_providers_kind_valid;
INSERT INTO llm_providers (id, kind, base_url, api_key_ref, enabled, status)
VALUES ('openrouter', 'openai_api', 'https://openrouter.ai/api/v1', 'OPENROUTER_API_KEY', true, 'unknown');

UPDATE llm_task_profiles
SET provider_id = 'openrouter', model = 'deepseek/deepseek-v4-flash-0731',
    reasoning_effort = NULL,
    timeout_seconds = CASE
      WHEN task IN ('classify', 'narration_framing') THEN 120
      WHEN task = 'watch_tower_retrieval' THEN 20
      WHEN task = 'watch_tower_answer' THEN 45
      WHEN task = 'content_cleaning' THEN 300
      ELSE 180
    END;

DELETE FROM llm_task_profiles WHERE task = 'admin_article_analysis';
DELETE FROM llm_providers WHERE id = 'codex_cli';
ALTER TABLE llm_providers ADD CONSTRAINT llm_providers_kind_valid CHECK (kind = 'openai_api');
ALTER TABLE llm_task_profiles DROP CONSTRAINT llm_task_profiles_reasoning_effort_valid;
-- Retain the pre-existing nullable column so version 51 and older migrations
-- remain usable after rollback.

COMMIT;
