BEGIN;

UPDATE llm_task_profiles
SET provider_id = 'openrouter',
    model = 'google/gemini-2.5-flash-lite',
    reasoning_effort = NULL,
    temperature = 0,
    enabled = true
WHERE task IN ('watch_tower_retrieval', 'watch_tower_answer');

DROP TABLE IF EXISTS watch_tower_settings;

COMMIT;
