BEGIN;

DELETE FROM llm_task_profiles WHERE task IN ('watch_tower_retrieval', 'watch_tower_answer');
DROP TABLE IF EXISTS watch_tower_messages;
DROP TABLE IF EXISTS watch_tower_threads;

COMMIT;
