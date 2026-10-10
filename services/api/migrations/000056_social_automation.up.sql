BEGIN;

-- Instant selection when new coverage arrives, and the AI quality gate for auto-publishing.
ALTER TABLE social_settings
  ADD COLUMN realtime_enabled boolean NOT NULL DEFAULT true,
  ADD COLUMN min_review_score integer NOT NULL DEFAULT 80 CHECK (min_review_score BETWEEN 0 AND 100);

-- Every post belongs to one channel. Only Facebook publishes today; the column keeps the
-- content calendar and log ready for more channels.
ALTER TABLE social_posts
  ADD COLUMN channel text NOT NULL DEFAULT 'facebook' CHECK (channel IN ('facebook')),
  ADD COLUMN trigger text NOT NULL DEFAULT 'scheduled' CHECK (trigger IN ('scheduled', 'realtime', 'manual')),
  ADD COLUMN review_status text NOT NULL DEFAULT 'not_reviewed' CHECK (review_status IN ('not_reviewed', 'passed', 'failed', 'unavailable')),
  ADD COLUMN review_score integer CHECK (review_score BETWEEN 0 AND 100),
  ADD COLUMN review_summary text NOT NULL DEFAULT '' CHECK (char_length(review_summary) <= 1000),
  ADD COLUMN review_issues jsonb NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(review_issues) = 'array'),
  ADD COLUMN review_llm_call_id bigint REFERENCES llm_calls(id) ON DELETE SET NULL,
  ADD COLUMN reviewed_at timestamptz;

UPDATE social_posts SET trigger = 'manual' WHERE created_by <> 'selector';

CREATE INDEX social_posts_calendar ON social_posts (channel, (COALESCE(published_at, scheduled_for, created_at)));

INSERT INTO llm_task_profiles (task, provider_id, model, reasoning_effort, timeout_seconds, enabled)
VALUES ('social_post_review', 'codex_cli', 'gpt-6.1-sol', 'medium', 300, true);
INSERT INTO agent_workflows (task, name, purpose, category, personality, tone, response_language, audience)
VALUES ('social_post_review', 'Social post quality review', 'Checks every drafted social post against the evidence and the newsroom standards before it may publish automatically.',
        'Social publishing', 'Strict senior Sinhala news editor and fact checker.',
        'exacting and neutral', 'en', 'Thulawa editors');
INSERT INTO agent_workflow_versions (task, revision, snapshot)
SELECT task, revision, jsonb_build_object(
  'custom_instructions', custom_instructions, 'personality', personality,
  'learning_notes', learning_notes, 'tone', tone, 'response_language', response_language,
  'audience', audience, 'enabled', enabled
) FROM agent_workflows WHERE task = 'social_post_review';

COMMIT;
