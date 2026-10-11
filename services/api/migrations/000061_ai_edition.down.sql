BEGIN;
DELETE FROM agent_workflow_versions WHERE task IN ('edition_layout', 'story_report', 'story_report_review');
DELETE FROM agent_workflows WHERE task IN ('edition_layout', 'story_report', 'story_report_review');
DELETE FROM llm_task_profiles WHERE task IN ('edition_layout', 'story_report', 'story_report_review');
DROP TABLE IF EXISTS editions;
DROP TABLE IF EXISTS story_reports;
COMMIT;
