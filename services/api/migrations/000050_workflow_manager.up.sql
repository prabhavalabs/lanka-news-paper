BEGIN;

CREATE TABLE agent_workflows (
  task text PRIMARY KEY,
  name text NOT NULL,
  purpose text NOT NULL,
  category text NOT NULL,
  custom_instructions text NOT NULL DEFAULT '',
  personality text NOT NULL DEFAULT '',
  learning_notes text NOT NULL DEFAULT '',
  tone text NOT NULL DEFAULT 'neutral',
  response_language text NOT NULL DEFAULT 'source',
  audience text NOT NULL DEFAULT 'general',
  enabled boolean NOT NULL DEFAULT true,
  revision integer NOT NULL DEFAULT 1,
  updated_by uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT agent_workflow_text_lengths CHECK (
    length(name) BETWEEN 1 AND 120
    AND length(purpose) BETWEEN 1 AND 500
    AND length(custom_instructions) <= 12000
    AND length(personality) <= 3000
    AND length(learning_notes) <= 12000
    AND length(tone) BETWEEN 1 AND 80
    AND length(response_language) BETWEEN 1 AND 40
    AND length(audience) BETWEEN 1 AND 160
  ),
  CONSTRAINT agent_workflow_revision_positive CHECK (revision > 0)
);

CREATE TRIGGER agent_workflows_set_updated_at
BEFORE UPDATE ON agent_workflows
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

INSERT INTO agent_workflows (task, name, purpose, category, custom_instructions, personality, tone, response_language, audience)
VALUES
  ('content_cleaning', 'Content cleaning', 'Turns captured publisher pages into faithful, readable Markdown.', 'Editorial pipeline', '', 'Precise copy editor who preserves the source language and meaning.', 'clear and restrained', 'source', 'newsroom editors'),
  ('article_summary', 'Article summary', 'Creates factual summaries and key points from cleaned articles.', 'Editorial pipeline', '', 'Careful wire-service editor focused on verified facts and useful context.', 'concise and neutral', 'source', 'general news readers'),
  ('classify', 'Classification', 'Assigns the most appropriate newsroom category.', 'Editorial pipeline', '', 'Consistent newsroom taxonomist.', 'neutral', 'source', 'newsroom editors'),
  ('event_synthesis', 'Event synthesis', 'Combines multiple reports about one event without flattening source differences.', 'Editorial pipeline', '', 'Evidence-led synthesis editor who distinguishes agreement from disagreement.', 'balanced and precise', 'source', 'general news readers'),
  ('narration_framing', 'Narration analysis', 'Identifies political-economic framing and supporting evidence.', 'Intelligence', '', 'Non-partisan media analyst who separates evidence from interpretation.', 'analytical and neutral', 'source', 'newsroom editors'),
  ('admin_article_analysis', 'Administrative analysis', 'Produces the detailed structured analysis used by editorial review tools.', 'Intelligence', '', 'Rigorous senior research editor.', 'analytical and precise', 'source', 'newsroom administrators'),
  ('watch_tower_retrieval', 'Watch Tower retrieval', 'Builds bilingual search plans for newsroom questions.', 'Watch Tower', '', 'Fast research librarian familiar with Sri Lankan news terminology.', 'direct', 'source', 'newsroom editors'),
  ('watch_tower_answer', 'Watch Tower answers', 'Synthesizes cited answers from retrieved newsroom evidence.', 'Watch Tower', '', 'Skeptical research analyst who makes uncertainty visible.', 'clear and evidence-led', 'source', 'newsroom editors'),
  ('newsletter_editorial', 'Morning newsletter', 'Prepares the last-24-hours email edition from rights-approved published stories.', 'Newsletter', 'Prioritize material public-interest developments. Preserve names, numbers, dates, and attribution. Make the brief easy to scan on a phone.', 'Warm, calm Sinhala morning editor; authoritative without sounding bureaucratic or sensational.', 'warm, concise, and neutral', 'si', 'busy Sri Lankan readers')
ON CONFLICT (task) DO NOTHING;

CREATE TABLE agent_workflow_versions (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  task text NOT NULL REFERENCES agent_workflows(task) ON DELETE CASCADE,
  revision integer NOT NULL,
  snapshot jsonb NOT NULL,
  created_by uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  UNIQUE (task, revision),
  CONSTRAINT agent_workflow_version_snapshot_object CHECK (jsonb_typeof(snapshot) = 'object')
);

INSERT INTO agent_workflow_versions (task, revision, snapshot)
SELECT task, revision, jsonb_build_object(
  'custom_instructions', custom_instructions,
  'personality', personality,
  'learning_notes', learning_notes,
  'tone', tone,
  'response_language', response_language,
  'audience', audience,
  'enabled', enabled
)
FROM agent_workflows;

CREATE TABLE agent_feedback (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_task text NOT NULL REFERENCES agent_workflows(task) ON DELETE CASCADE,
  rating text NOT NULL,
  category text NOT NULL,
  message text NOT NULL,
  status text NOT NULL DEFAULT 'new',
  created_by uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  reviewed_by uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  reviewed_at timestamptz,
  CONSTRAINT agent_feedback_rating_valid CHECK (rating IN ('helpful', 'needs_improvement')),
  CONSTRAINT agent_feedback_category_valid CHECK (category IN ('accuracy', 'tone', 'relevance', 'formatting', 'safety', 'other')),
  CONSTRAINT agent_feedback_status_valid CHECK (status IN ('new', 'reviewed', 'applied', 'dismissed')),
  CONSTRAINT agent_feedback_message_length CHECK (length(btrim(message)) BETWEEN 3 AND 3000)
);

CREATE INDEX agent_feedback_status_created
  ON agent_feedback (status, created_at DESC);
CREATE INDEX agent_feedback_workflow_created
  ON agent_feedback (workflow_task, created_at DESC);

ALTER TABLE newsletter_settings
  ADD COLUMN runtime_bootstrapped boolean NOT NULL DEFAULT false,
  ADD COLUMN max_stories smallint NOT NULL DEFAULT 30,
  ADD COLUMN lead_story_count smallint NOT NULL DEFAULT 5,
  ADD COLUMN subject_template text NOT NULL DEFAULT 'උදෑසන පුවත් සංග්‍රහය — {{date}}',
  ADD COLUMN preheader_template text NOT NULL DEFAULT 'පසුගිය පැය 24: පුවත් {{articles}} · සිදුවීම් {{events}} · මූලාශ්‍ර {{sources}}',
  ADD COLUMN intro_text text NOT NULL DEFAULT 'ඔබ දැනගත යුතු පසුගිය පැය 24 හි වැදගත් පුවත් මෙන්න.',
  ADD COLUMN footer_text text NOT NULL DEFAULT 'මෙම සංග්‍රහය ප්‍රකාශිත පුවත් සහ මූලාශ්‍ර-අතර සාරාංශ මත ස්වයංක්‍රීයව සකස් කර ඇත. සම්පූර්ණ විස්තර සඳහා සබැඳි විවෘත කරන්න.',
  ADD CONSTRAINT newsletter_max_stories_valid CHECK (max_stories BETWEEN 1 AND 50),
  ADD CONSTRAINT newsletter_lead_count_valid CHECK (lead_story_count BETWEEN 1 AND 10 AND lead_story_count <= max_stories),
  ADD CONSTRAINT newsletter_template_lengths CHECK (
    length(subject_template) BETWEEN 1 AND 240
    AND length(preheader_template) BETWEEN 1 AND 500
    AND length(intro_text) <= 1200
    AND length(footer_text) <= 2000
  );

INSERT INTO llm_task_profiles (
  task, provider_id, model, reasoning_effort, max_output_tokens,
  temperature, timeout_seconds, enabled
)
SELECT 'newsletter_editorial', profile.provider_id, profile.model, profile.reasoning_effort,
       5000, profile.temperature, GREATEST(profile.timeout_seconds, 180), profile.enabled
FROM llm_task_profiles profile
WHERE profile.task = 'article_summary'
ON CONFLICT (task) DO UPDATE SET
  provider_id = EXCLUDED.provider_id,
  model = EXCLUDED.model,
  reasoning_effort = EXCLUDED.reasoning_effort,
  max_output_tokens = EXCLUDED.max_output_tokens,
  temperature = EXCLUDED.temperature,
  timeout_seconds = EXCLUDED.timeout_seconds,
  enabled = EXCLUDED.enabled;

COMMIT;
