BEGIN;

UPDATE source_endpoints AS endpoint
SET url = 'https://sinhala.adaderana.lk/rsshotnews.php',
    paused = false,
    health_state = 'unknown',
    last_error = NULL,
    etag = NULL,
    last_modified = NULL,
    consecutive_failures = 0,
    backoff_until = NULL,
    verified_official = true
FROM sources AS source
WHERE endpoint.source_id = source.id
  AND source.website = 'https://sinhala.adaderana.lk'
  AND endpoint.url = 'https://sinhala.adaderana.lk/rss.xml';

UPDATE source_endpoints AS endpoint
SET paused = false,
    health_state = 'unknown',
    last_error = NULL,
    consecutive_failures = 0,
    backoff_until = NULL
FROM sources AS source
WHERE endpoint.source_id = source.id
  AND source.website = 'https://www.lankadeepa.lk'
  AND endpoint.url IN (
    'https://www.lankadeepa.lk/rss/latest_news/1',
    'https://www.lankadeepa.lk/RSS_Feeds/latest_news'
  );

COMMIT;
