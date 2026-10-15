BEGIN;
CREATE TABLE storylines (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
  title_si text NOT NULL CHECK (char_length(title_si) BETWEEN 1 AND 120),
  summary_si text NOT NULL DEFAULT '' CHECK (char_length(summary_si) <= 400),
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
  pinned boolean NOT NULL DEFAULT false,
  hidden boolean NOT NULL DEFAULT false,
  trend_score int NOT NULL DEFAULT 0,
  trending boolean NOT NULL DEFAULT false,
  metrics jsonb NOT NULL DEFAULT '{}',
  first_seen_at timestamptz NOT NULL DEFAULT now(),
  last_activity_at timestamptz NOT NULL DEFAULT now(),
  trending_since timestamptz,
  title_locked boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER storylines_set_updated_at BEFORE UPDATE ON storylines
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX storylines_trending_idx ON storylines (pinned DESC, trend_score DESC) WHERE trending AND NOT hidden AND status='active';
CREATE TABLE storyline_events (
  storyline_id uuid NOT NULL REFERENCES storylines(id) ON DELETE CASCADE,
  event_id uuid PRIMARY KEY REFERENCES event_clusters(id) ON DELETE CASCADE,
  added_at timestamptz NOT NULL DEFAULT now(),
  added_by text NOT NULL CHECK (added_by IN ('ai','admin'))
);
CREATE INDEX storyline_events_storyline_idx ON storyline_events(storyline_id);
ALTER TABLE social_posts DROP CONSTRAINT social_posts_kind_check;
ALTER TABLE social_posts ADD CONSTRAINT social_posts_kind_check CHECK (kind IN ('breaking','top_story','update'));
INSERT INTO llm_task_profiles (task, provider_id, model, reasoning_effort, timeout_seconds, enabled)
VALUES ('storyline_grouping', 'codex_cli', 'gpt-6-luna', 'medium', 180, true)
ON CONFLICT (task) DO NOTHING;
INSERT INTO agent_workflows (task, name, purpose, category, personality, tone, response_language, audience)
VALUES ('storyline_grouping', 'Storyline grouping', 'Groups distinct developments of an ongoing news saga into a running storyline.',
        'Editorial pipeline', 'Careful news desk editor who recognises ongoing stories while keeping unrelated subjects apart.',
        'precise', 'en', 'newsroom editors')
ON CONFLICT (task) DO NOTHING;
INSERT INTO agent_workflow_versions (task, revision, snapshot)
SELECT task, revision, jsonb_build_object(
  'custom_instructions', custom_instructions, 'personality', personality,
  'learning_notes', learning_notes, 'tone', tone, 'response_language', response_language,
  'audience', audience, 'enabled', enabled
) FROM agent_workflows WHERE task = 'storyline_grouping'
ON CONFLICT DO NOTHING;
COMMIT;
