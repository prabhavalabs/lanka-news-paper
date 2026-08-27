BEGIN;

DELETE FROM llm_task_profiles WHERE task = 'newsletter_editorial';
ALTER TABLE newsletter_settings
  DROP CONSTRAINT IF EXISTS newsletter_template_lengths,
  DROP CONSTRAINT IF EXISTS newsletter_lead_count_valid,
  DROP CONSTRAINT IF EXISTS newsletter_max_stories_valid,
  DROP COLUMN IF EXISTS footer_text,
  DROP COLUMN IF EXISTS intro_text,
  DROP COLUMN IF EXISTS preheader_template,
  DROP COLUMN IF EXISTS subject_template,
  DROP COLUMN IF EXISTS lead_story_count,
  DROP COLUMN IF EXISTS max_stories,
  DROP COLUMN IF EXISTS runtime_bootstrapped;
DROP TABLE IF EXISTS agent_feedback;
DROP TABLE IF EXISTS agent_workflow_versions;
DROP TRIGGER IF EXISTS agent_workflows_set_updated_at ON agent_workflows;
DROP TABLE IF EXISTS agent_workflows;

COMMIT;
