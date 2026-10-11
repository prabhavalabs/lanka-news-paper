BEGIN;

-- Complete reports the AI desk writes for front-page stories, sized to their block. A report
-- is reused while its story has not changed (same update time and outlet count).
CREATE TABLE story_reports (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  event_id uuid REFERENCES event_clusters(id) ON DELETE CASCADE,
  article_id uuid REFERENCES articles(id) ON DELETE CASCADE,
  size text NOT NULL CHECK (size IN ('lead', 'feature', 'standard', 'short', 'brief')),
  headline text NOT NULL,
  standfirst text NOT NULL DEFAULT '',
  paragraphs jsonb NOT NULL CHECK (jsonb_typeof(paragraphs) = 'array'),
  word_count integer NOT NULL DEFAULT 0,
  illustration_prompt text NOT NULL DEFAULT '',
  story_updated_at timestamptz NOT NULL,
  source_count integer NOT NULL,
  review_score integer,
  review_summary text NOT NULL DEFAULT '',
  revisions integer NOT NULL DEFAULT 0,
  llm_call_id bigint REFERENCES llm_calls(id) ON DELETE SET NULL,
  image_key text,
  image_alt text,
  image_width integer,
  image_height integer,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CHECK ((event_id IS NULL) <> (article_id IS NULL))
);
CREATE INDEX story_reports_event ON story_reports (event_id, created_at DESC) WHERE event_id IS NOT NULL;
CREATE INDEX story_reports_article ON story_reports (article_id, created_at DESC) WHERE article_id IS NOT NULL;

-- Each edition is the AI layout editor's arrangement of the front page.
CREATE TABLE editions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  status text NOT NULL DEFAULT 'building' CHECK (status IN ('building', 'published', 'failed')),
  blocks jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(blocks) = 'array'),
  fingerprint text NOT NULL DEFAULT '',
  layout_note text NOT NULL DEFAULT '',
  error text,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  published_at timestamptz
);
CREATE INDEX editions_published ON editions (published_at DESC) WHERE status = 'published';

INSERT INTO llm_task_profiles (task, provider_id, model, reasoning_effort, timeout_seconds, enabled) VALUES
  ('edition_layout', 'codex_cli', 'gpt-6.1-sol', 'medium', 300, true),
  ('story_report', 'codex_cli', 'gpt-6.1-sol', 'medium', 300, true),
  ('story_report_review', 'codex_cli', 'gpt-6.1-sol', 'medium', 300, true);

INSERT INTO agent_workflows (task, name, purpose, category, personality, tone, response_language, audience) VALUES
  ('edition_layout', 'Front page layout editor', 'Chooses the stories, block sizes and illustration places for each automatic front-page edition.',
   'Front page', 'Experienced Sri Lankan broadsheet chief sub-editor.', 'decisive and neutral', 'en', 'Thulawa readers'),
  ('story_report', 'Front page report writer', 'Writes complete original Sinhala reports for front-page stories from every outlet''s coverage, sized to their block.',
   'Front page', 'Senior Sinhala newspaper reporter and sub-editor.', 'clear, factual and neutral', 'si', 'Sinhala newspaper readers'),
  ('story_report_review', 'Front page report review', 'Checks each written report against the evidence before it is printed on the front page.',
   'Front page', 'Strict senior Sinhala news editor and fact checker.', 'exacting and neutral', 'en', 'Thulawa editors');

INSERT INTO agent_workflow_versions (task, revision, snapshot)
SELECT task, revision, jsonb_build_object(
  'custom_instructions', custom_instructions, 'personality', personality,
  'learning_notes', learning_notes, 'tone', tone, 'response_language', response_language,
  'audience', audience, 'enabled', enabled
) FROM agent_workflows WHERE task IN ('edition_layout', 'story_report', 'story_report_review');

COMMIT;
