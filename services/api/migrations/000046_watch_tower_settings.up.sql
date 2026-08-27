BEGIN;

CREATE TABLE watch_tower_settings (
  singleton boolean PRIMARY KEY DEFAULT true,
  response_language text NOT NULL DEFAULT 'si',
  updated_by uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT watch_tower_settings_singleton CHECK (singleton),
  CONSTRAINT watch_tower_response_language_valid CHECK (response_language IN ('si', 'en'))
);

INSERT INTO watch_tower_settings (singleton, response_language)
VALUES (true, 'si');

UPDATE llm_task_profiles
SET provider_id = 'openrouter',
    model = 'deepseek/deepseek-v4-flash-0731',
    reasoning_effort = NULL,
    temperature = 0,
    enabled = true
WHERE task IN ('watch_tower_retrieval', 'watch_tower_answer');

COMMIT;
