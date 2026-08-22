BEGIN;

UPDATE source_endpoints AS endpoint
SET url = 'https://sinhala.adaderana.lk/rss.xml',
    paused = true,
    health_state = 'failed',
    last_error = 'Official Ada Derana RSS feed returns HTTP 403 to the production worker egress IP; publisher allowlisting or an approved alternate delivery path is required',
    etag = NULL,
    last_modified = NULL,
    consecutive_failures = 0,
    backoff_until = NULL,
    verified_official = true
FROM sources AS source
WHERE endpoint.source_id = source.id
  AND source.website = 'https://sinhala.adaderana.lk'
  AND endpoint.url = 'https://sinhala.adaderana.lk/rsshotnews.php';

UPDATE source_endpoints AS endpoint
SET paused = true,
    health_state = 'failed',
    last_error = 'Official Lankadeepa RSS endpoint accepted the request but returned no response before the 20-second ingestion timeout',
    backoff_until = NULL
FROM sources AS source
WHERE endpoint.source_id = source.id
  AND source.website = 'https://www.lankadeepa.lk'
  AND endpoint.url IN (
    'https://www.lankadeepa.lk/rss/latest_news/1',
    'https://www.lankadeepa.lk/RSS_Feeds/latest_news'
  );

COMMIT;
