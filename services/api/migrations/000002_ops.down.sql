BEGIN;

DELETE FROM llm_calls;
DELETE FROM llm_task_profiles;
DELETE FROM llm_providers;
DELETE FROM daily_briefs;
DELETE FROM event_articles;
DELETE FROM event_clusters;
DELETE FROM article_versions;
DELETE FROM articles;
DELETE FROM rights_profiles;
DELETE FROM ingestion_runs;
DELETE FROM source_endpoints;
DELETE FROM sources WHERE name IN ('BBC News Sinhala', 'ITN News', 'Lankadeepa', 'Ada.lk');
DELETE FROM admin_sessions;

ALTER TABLE articles DROP COLUMN IF EXISTS event_id;
ALTER TABLE articles DROP COLUMN IF EXISTS publisher_category;
ALTER TABLE articles DROP COLUMN IF EXISTS category_id;
ALTER TABLE event_clusters DROP COLUMN IF EXISTS is_breaking;
ALTER TABLE source_endpoints DROP COLUMN IF EXISTS last_error;
ALTER TABLE source_endpoints DROP COLUMN IF EXISTS last_success_at;
ALTER TABLE source_endpoints DROP COLUMN IF EXISTS last_modified;
ALTER TABLE source_endpoints DROP COLUMN IF EXISTS etag;
DROP TABLE IF EXISTS admin_sessions;
ALTER TABLE admin_users DROP COLUMN IF EXISTS totp_secret;
ALTER TABLE admin_users DROP COLUMN IF EXISTS password_hash;
DROP TABLE IF EXISTS llm_calls;
DROP TABLE IF EXISTS llm_task_profiles;
DROP TABLE IF EXISTS llm_providers;
DROP TABLE IF EXISTS daily_briefs;

COMMIT;
