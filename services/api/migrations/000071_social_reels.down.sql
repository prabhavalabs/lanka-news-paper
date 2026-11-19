BEGIN;
DELETE FROM agent_workflow_versions WHERE task IN ('reel_story_pick', 'reel_script');
DELETE FROM agent_workflows WHERE task IN ('reel_story_pick', 'reel_script');
DELETE FROM llm_task_profiles WHERE task IN ('reel_story_pick', 'reel_script');
DROP TABLE IF EXISTS social_reels;
COMMIT;
