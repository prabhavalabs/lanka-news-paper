BEGIN;

CREATE TEMP TABLE additional_source_ids ON COMMIT DROP AS
SELECT id
FROM sources
WHERE website IN (
  'https://www.gossiplankanews.com',
  'https://www.meepura.com',
  'https://www.infosrilanka.lk',
  'https://rata.lk',
  'https://aithiya.lk',
  'https://medialk.com',
  'https://www.news19.lk',
  'https://www.gossiplankahotnews.com',
  'https://supirigossip.com',
  'https://sarasaviya.lk',
  'https://pmd.gov.lk/si'
);

CREATE TEMP TABLE additional_endpoint_ids ON COMMIT DROP AS
SELECT id FROM source_endpoints WHERE source_id IN (SELECT id FROM additional_source_ids);

CREATE TEMP TABLE additional_article_ids ON COMMIT DROP AS
SELECT id FROM articles WHERE endpoint_id IN (SELECT id FROM additional_endpoint_ids);

DELETE FROM event_articles WHERE article_id IN (SELECT id FROM additional_article_ids);
DELETE FROM article_versions WHERE article_id IN (SELECT id FROM additional_article_ids);
DELETE FROM articles WHERE id IN (SELECT id FROM additional_article_ids);
DELETE FROM quarantine_payloads WHERE endpoint_id IN (SELECT id FROM additional_endpoint_ids);
DELETE FROM ingestion_runs WHERE endpoint_id IN (SELECT id FROM additional_endpoint_ids);
DELETE FROM rights_profiles WHERE source_id IN (SELECT id FROM additional_source_ids);
DELETE FROM source_endpoints WHERE source_id IN (SELECT id FROM additional_source_ids);
DELETE FROM sources WHERE id IN (SELECT id FROM additional_source_ids);

COMMIT;
