BEGIN;

CREATE TEMP TABLE catalog_source_ids ON COMMIT DROP AS
SELECT id
FROM sources
WHERE website IN (
  'https://www.divaina.lk',
  'https://www.vikalpa.org',
  'https://siyathanews.lk',
  'https://sinhala.srilankamirror.com',
  'https://www.anidda.lk',
  'https://dasathalankanews.com',
  'https://lankasara.com/si'
);

CREATE TEMP TABLE catalog_endpoint_ids ON COMMIT DROP AS
SELECT endpoint.id
FROM source_endpoints AS endpoint
WHERE endpoint.source_id IN (SELECT id FROM catalog_source_ids)
   OR endpoint.url IN (
     'https://www.lankadeepa.lk/rss/latest_news/1',
     'https://sinhala.adaderana.lk/rsshotnews.php'
   );

CREATE TEMP TABLE catalog_article_ids ON COMMIT DROP AS
SELECT id FROM articles WHERE endpoint_id IN (SELECT id FROM catalog_endpoint_ids);

DELETE FROM event_articles WHERE article_id IN (SELECT id FROM catalog_article_ids);
DELETE FROM article_versions WHERE article_id IN (SELECT id FROM catalog_article_ids);
DELETE FROM articles WHERE id IN (SELECT id FROM catalog_article_ids);
DELETE FROM quarantine_payloads WHERE endpoint_id IN (SELECT id FROM catalog_endpoint_ids);
DELETE FROM ingestion_runs WHERE endpoint_id IN (SELECT id FROM catalog_endpoint_ids);
DELETE FROM rights_profiles WHERE source_id IN (SELECT id FROM catalog_source_ids);
DELETE FROM source_endpoints WHERE source_id IN (SELECT id FROM catalog_source_ids);
DELETE FROM sources WHERE id IN (SELECT id FROM catalog_source_ids);

UPDATE source_endpoints AS endpoint
SET url = 'https://www.lankadeepa.lk/rss',
    polling_interval_seconds = 300,
    verified_official = false,
    paused = true,
    health_state = 'unknown',
    last_error = NULL
FROM sources AS source
WHERE endpoint.source_id = source.id
  AND source.website = 'https://www.lankadeepa.lk';

UPDATE sources
SET active = false,
    description = ''
WHERE website = 'https://www.lankadeepa.lk';

UPDATE source_endpoints AS endpoint
SET url = 'https://www.ada.lk/rss',
    polling_interval_seconds = 300,
    verified_official = false,
    paused = true,
    health_state = 'unknown',
    last_error = NULL
FROM sources AS source
WHERE endpoint.source_id = source.id
  AND source.website = 'https://sinhala.adaderana.lk';

UPDATE sources
SET name = 'Ada.lk',
    legal_name = 'Ada Derana / TV Derana',
    website = 'https://www.ada.lk',
    active = false,
    description = ''
WHERE website = 'https://sinhala.adaderana.lk';

UPDATE sources
SET active = true,
    description = 'ඉන්ද්‍රජාල රූපවාහිනී ජාලයේ ප්‍රවෘත්ති සේවය.'
WHERE website = 'https://www.itnnews.lk';

UPDATE source_endpoints AS endpoint
SET verified_official = true,
    paused = false,
    health_state = 'unknown',
    last_error = NULL
FROM sources AS source
WHERE endpoint.source_id = source.id
  AND source.website = 'https://www.itnnews.lk';

COMMIT;
