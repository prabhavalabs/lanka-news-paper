BEGIN;
DELETE FROM agent_workflow_versions WHERE task = 'social_post_review';
DELETE FROM agent_workflows WHERE task = 'social_post_review';
DELETE FROM llm_task_profiles WHERE task = 'social_post_review';
DROP INDEX IF EXISTS social_posts_calendar;
ALTER TABLE social_posts
  DROP COLUMN channel, DROP COLUMN trigger, DROP COLUMN review_status, DROP COLUMN review_score,
  DROP COLUMN review_summary, DROP COLUMN review_issues, DROP COLUMN review_llm_call_id, DROP COLUMN reviewed_at;
ALTER TABLE social_settings DROP COLUMN realtime_enabled, DROP COLUMN min_review_score;
COMMIT;
