BEGIN;

DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS admin_users;
DROP TABLE IF EXISTS complaints;
DROP TABLE IF EXISTS editorial_actions;
DROP TABLE IF EXISTS event_articles;
DROP TABLE IF EXISTS event_clusters;
DROP TABLE IF EXISTS article_versions;
DROP TABLE IF EXISTS articles;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS ingestion_runs;
DROP TABLE IF EXISTS rights_profiles;
DROP TABLE IF EXISTS source_endpoints;
DROP TABLE IF EXISTS sources;
DROP FUNCTION IF EXISTS set_updated_at();

COMMIT;
