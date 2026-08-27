BEGIN;

CREATE TABLE watch_tower_threads (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
  title text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT watch_tower_thread_title_not_blank CHECK (length(btrim(title)) > 0)
);

CREATE INDEX watch_tower_threads_user_updated
  ON watch_tower_threads (user_id, updated_at DESC);

CREATE TABLE watch_tower_messages (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  thread_id uuid NOT NULL REFERENCES watch_tower_threads(id) ON DELETE CASCADE,
  role text NOT NULL,
  content text NOT NULL,
  citations jsonb NOT NULL DEFAULT '[]'::jsonb,
  suggestions jsonb NOT NULL DEFAULT '[]'::jsonb,
  provider_id text,
  provider_model text,
  search_label text,
  search_from timestamptz,
  search_to timestamptz,
  search_article_count integer,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT watch_tower_message_role_valid CHECK (role IN ('user', 'assistant')),
  CONSTRAINT watch_tower_message_content_not_blank CHECK (length(btrim(content)) > 0),
  CONSTRAINT watch_tower_message_citations_array CHECK (jsonb_typeof(citations) = 'array'),
  CONSTRAINT watch_tower_message_suggestions_array CHECK (jsonb_typeof(suggestions) = 'array'),
  CONSTRAINT watch_tower_message_search_complete CHECK (
    (search_label IS NULL AND search_from IS NULL AND search_to IS NULL AND search_article_count IS NULL)
    OR (search_label IS NOT NULL AND search_from IS NOT NULL AND search_to IS NOT NULL AND search_article_count >= 0)
  )
);

CREATE INDEX watch_tower_messages_thread_created
  ON watch_tower_messages (thread_id, created_at, id);

INSERT INTO llm_task_profiles (
  task, provider_id, model, reasoning_effort, max_output_tokens,
  temperature, timeout_seconds, enabled
)
SELECT requested.task, provider.id, 'google/gemini-2.5-flash-lite', NULL,
       requested.max_output_tokens, 0, requested.timeout_seconds, provider.enabled
FROM llm_providers provider
CROSS JOIN (VALUES
  ('watch_tower_retrieval', 300, 20),
  ('watch_tower_answer', 1800, 45)
) AS requested(task, max_output_tokens, timeout_seconds)
WHERE provider.id = 'openrouter'
ON CONFLICT (task) DO UPDATE SET
  provider_id = EXCLUDED.provider_id,
  model = EXCLUDED.model,
  reasoning_effort = EXCLUDED.reasoning_effort,
  max_output_tokens = EXCLUDED.max_output_tokens,
  temperature = EXCLUDED.temperature,
  timeout_seconds = EXCLUDED.timeout_seconds,
  enabled = EXCLUDED.enabled;

COMMIT;
