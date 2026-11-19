BEGIN;
DELETE FROM agent_workflow_versions WHERE task IN ('trend_research', 'trend_radar');
DELETE FROM agent_workflows WHERE task IN ('trend_research', 'trend_radar');
DELETE FROM llm_task_profiles WHERE task IN ('trend_research', 'trend_radar');
DROP TABLE IF EXISTS trend_topics;
DROP TABLE IF EXISTS trend_radar_runs;
DROP TABLE IF EXISTS trend_youtube_videos;
DROP TABLE IF EXISTS trend_youtube_channels;
COMMIT;
