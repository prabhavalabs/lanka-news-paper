BEGIN;

ALTER TABLE admin_analysis_backfills
  ADD COLUMN workflow text NOT NULL DEFAULT 'single_pass';

ALTER TABLE admin_analysis_backfills
  DROP CONSTRAINT admin_analysis_backfill_provider_valid,
  ADD CONSTRAINT admin_analysis_backfill_provider_valid
    CHECK (provider IN ('openrouter', 'codex_cli', 'pipeline')),
  ADD CONSTRAINT admin_analysis_backfill_workflow_valid
    CHECK (workflow IN ('single_pass', 'full_pipeline'));

CREATE TABLE article_analysis_documents (
  article_id uuid PRIMARY KEY REFERENCES articles(id) ON DELETE CASCADE,
  source_content_id uuid REFERENCES article_contents(id) ON DELETE CASCADE,
  original_text text NOT NULL,
  cleaned_text text NOT NULL,
  summary_text text NOT NULL DEFAULT '',
  summary_points jsonb NOT NULL DEFAULT '[]'::jsonb,
  cleaner_version text NOT NULL,
  summary_provider text NOT NULL DEFAULT '',
  summary_model text NOT NULL DEFAULT '',
  cleaned_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  summarized_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT article_analysis_original_not_blank CHECK (length(btrim(original_text)) > 0),
  CONSTRAINT article_analysis_cleaned_not_blank CHECK (length(btrim(cleaned_text)) > 0),
  CONSTRAINT article_analysis_summary_points_array CHECK (jsonb_typeof(summary_points) = 'array')
);

ALTER TABLE article_political_analysis
  ADD COLUMN left_probability numeric NOT NULL DEFAULT 0,
  ADD COLUMN center_probability numeric NOT NULL DEFAULT 1,
  ADD COLUMN right_probability numeric NOT NULL DEFAULT 0,
  ADD COLUMN axis_version text NOT NULL DEFAULT 'editorial-stance-v1',
  ADD CONSTRAINT article_political_left_probability_valid CHECK (left_probability BETWEEN 0 AND 1),
  ADD CONSTRAINT article_political_center_probability_valid CHECK (center_probability BETWEEN 0 AND 1),
  ADD CONSTRAINT article_political_right_probability_valid CHECK (right_probability BETWEEN 0 AND 1),
  ADD CONSTRAINT article_political_probability_total_valid CHECK (
    abs((left_probability + center_probability + right_probability) - 1) <= 0.02
  );

UPDATE article_political_analysis
SET left_probability = CASE label
      WHEN 'left' THEN 0.80 WHEN 'center_left' THEN 0.48 ELSE 0.05 END,
    center_probability = CASE label
      WHEN 'neutral' THEN 0.90 WHEN 'unclear' THEN 0.90
      WHEN 'center_left' THEN 0.47 WHEN 'center_right' THEN 0.47 ELSE 0.15 END,
    right_probability = CASE label
      WHEN 'right' THEN 0.80 WHEN 'center_right' THEN 0.48 ELSE 0.05 END;

CREATE TABLE event_narrative_analyses (
  event_id uuid PRIMARY KEY REFERENCES event_clusters(id) ON DELETE CASCADE,
  summary text NOT NULL,
  article_count integer NOT NULL,
  source_count integer NOT NULL,
  rated_source_count integer NOT NULL,
  left_percentage numeric NOT NULL,
  center_percentage numeric NOT NULL,
  right_percentage numeric NOT NULL,
  source_spectrum jsonb NOT NULL DEFAULT '[]'::jsonb,
  provider_id text NOT NULL,
  provider_model text NOT NULL,
  schema_version text NOT NULL DEFAULT 'event-narrative-v1',
  analyzed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT event_narrative_article_count_positive CHECK (article_count > 0),
  CONSTRAINT event_narrative_source_count_positive CHECK (source_count > 0),
  CONSTRAINT event_narrative_rated_source_count_valid CHECK (
    rated_source_count >= 0 AND rated_source_count <= source_count
  ),
  CONSTRAINT event_narrative_left_valid CHECK (left_percentage BETWEEN 0 AND 100),
  CONSTRAINT event_narrative_center_valid CHECK (center_percentage BETWEEN 0 AND 100),
  CONSTRAINT event_narrative_right_valid CHECK (right_percentage BETWEEN 0 AND 100),
  CONSTRAINT event_narrative_percentage_total_valid CHECK (
    (rated_source_count = 0 AND left_percentage + center_percentage + right_percentage = 0)
    OR (rated_source_count > 0 AND abs((left_percentage + center_percentage + right_percentage) - 100) <= 1)
  ),
  CONSTRAINT event_narrative_spectrum_array CHECK (jsonb_typeof(source_spectrum) = 'array')
);

INSERT INTO llm_task_profiles (
  task, provider_id, model, reasoning_effort, max_output_tokens,
  temperature, timeout_seconds, enabled
)
SELECT requested.task, profile.provider_id, profile.model, profile.reasoning_effort,
       requested.max_output_tokens, profile.temperature, GREATEST(profile.timeout_seconds, 180),
       profile.enabled
FROM llm_task_profiles profile
CROSS JOIN (VALUES
  ('article_summary', 1800),
  ('event_synthesis', 2200)
) AS requested(task, max_output_tokens)
WHERE profile.task = 'narration_framing'
ON CONFLICT (task) DO UPDATE SET
  provider_id = EXCLUDED.provider_id,
  model = EXCLUDED.model,
  reasoning_effort = EXCLUDED.reasoning_effort,
  timeout_seconds = EXCLUDED.timeout_seconds,
  max_output_tokens = EXCLUDED.max_output_tokens,
  temperature = EXCLUDED.temperature,
  enabled = EXCLUDED.enabled;

COMMIT;
