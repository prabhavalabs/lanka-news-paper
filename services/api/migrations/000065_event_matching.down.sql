BEGIN;
DELETE FROM agent_workflow_versions WHERE task = 'event_matching';
DELETE FROM agent_workflows WHERE task = 'event_matching';
DELETE FROM llm_task_profiles WHERE task = 'event_matching';
COMMIT;
