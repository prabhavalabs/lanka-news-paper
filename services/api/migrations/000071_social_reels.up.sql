BEGIN;
-- Facebook Reel drafts: built on the worker every six hours (or on demand) for a trending
-- storyline and held for an editor's approval. Video and cover files live in media storage.
CREATE TABLE social_reels (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  storyline_id uuid REFERENCES storylines(id) ON DELETE SET NULL,
  event_id uuid REFERENCES event_clusters(id) ON DELETE SET NULL,
  trigger text NOT NULL CHECK (trigger IN ('schedule', 'manual')),
  status text NOT NULL DEFAULT 'queued'
    CHECK (status IN ('queued', 'generating', 'ready', 'approved', 'rejected', 'posted', 'failed')),
  title text NOT NULL DEFAULT '',
  script jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(script) = 'object'),
  caption text NOT NULL DEFAULT '' CHECK (char_length(caption) <= 4000),
  assets jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(assets) = 'object'),
  trend jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(trend) = 'object'),
  video_key text,
  cover_key text,
  error text,
  requested_by uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  reviewed_by uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  reviewed_at timestamptz,
  rendered_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX social_reels_recent ON social_reels (created_at DESC);
CREATE INDEX social_reels_storyline ON social_reels (storyline_id, created_at DESC) WHERE storyline_id IS NOT NULL;

INSERT INTO llm_task_profiles (task, provider_id, model, reasoning_effort, timeout_seconds, enabled) VALUES
  ('reel_story_pick', 'codex_cli', 'gpt-6-luna', 'low', 180, true),
  ('reel_script', 'codex_cli', 'gpt-6.1-sol', 'medium', 300, true)
ON CONFLICT (task) DO NOTHING;
INSERT INTO agent_workflows (task, name, purpose, category, personality, tone, response_language, audience) VALUES
  ('reel_story_pick', 'Reel story pick', 'Ranks trending storylines for the next Facebook Reel, weighing coverage against what Sri Lankans are searching for on Google.',
   'Social publishing', 'Social media news editor who knows what Sri Lankan audiences care about today.', 'decisive and neutral', 'en', 'Thulawa editors'),
  ('reel_script', 'Reel script writer', 'Writes the Sinhala on-screen headline, key facts, a balanced debate question and the post text for a short Facebook Reel.',
   'Social publishing', 'Senior Sinhala news editor who writes punchy but strictly factual social video copy.', 'punchy, factual and neutral', 'si', 'Sinhala Facebook audience')
ON CONFLICT (task) DO NOTHING;
INSERT INTO agent_workflow_versions (task, revision, snapshot)
SELECT task, revision, jsonb_build_object(
  'custom_instructions', custom_instructions, 'personality', personality,
  'learning_notes', learning_notes, 'tone', tone, 'response_language', response_language,
  'audience', audience, 'enabled', enabled
) FROM agent_workflows WHERE task IN ('reel_story_pick', 'reel_script')
ON CONFLICT DO NOTHING;
COMMIT;
