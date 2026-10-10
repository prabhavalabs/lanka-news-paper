BEGIN;
DELETE FROM agent_workflows WHERE task = 'social_post_editorial';
DELETE FROM llm_task_profiles WHERE task = 'social_post_editorial';
DROP TABLE social_posts;
DROP TABLE social_oauth_states;
DROP TABLE social_accounts;
DROP TABLE social_settings;
COMMIT;
