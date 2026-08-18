BEGIN;

CREATE TEMP TABLE expanded_source_ids ON COMMIT DROP AS
SELECT id
FROM sources
WHERE website IN (
  'https://mawratanews.lk',
  'https://www.lankacnews.com',
  'https://www.colomboxnews.com',
  'https://sinhala.lankanewsweb.net',
  'https://hirunews.lk',
  'https://sinhala.newsfirst.lk',
  'https://www.dinamina.lk',
  'https://www.silumina.lk',
  'https://mawbima.lk',
  'https://aruna.lk',
  'https://monara.com',
  'https://nethnews.lk',
  'https://lankatruth.com/si',
  'https://sinhala.newswire.lk',
  'https://sinhala.asianmirror.lk',
  'https://sinhala.news.lk',
  'https://sinhala.lankapuvath.lk'
);

CREATE TEMP TABLE expanded_endpoint_ids ON COMMIT DROP AS
SELECT id FROM source_endpoints WHERE source_id IN (SELECT id FROM expanded_source_ids);

CREATE TEMP TABLE expanded_article_ids ON COMMIT DROP AS
SELECT id FROM articles WHERE endpoint_id IN (SELECT id FROM expanded_endpoint_ids);

DELETE FROM event_articles WHERE article_id IN (SELECT id FROM expanded_article_ids);
DELETE FROM article_versions WHERE article_id IN (SELECT id FROM expanded_article_ids);
DELETE FROM articles WHERE id IN (SELECT id FROM expanded_article_ids);
DELETE FROM quarantine_payloads WHERE endpoint_id IN (SELECT id FROM expanded_endpoint_ids);
DELETE FROM ingestion_runs WHERE endpoint_id IN (SELECT id FROM expanded_endpoint_ids);
DELETE FROM rights_profiles WHERE source_id IN (SELECT id FROM expanded_source_ids);
DELETE FROM source_endpoints WHERE source_id IN (SELECT id FROM expanded_source_ids);
DELETE FROM sources WHERE id IN (SELECT id FROM expanded_source_ids);

COMMIT;
