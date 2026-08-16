BEGIN;

UPDATE sources
SET active = true,
    description = 'Sinhala news and current-affairs publication.'
WHERE website = 'https://mawratanews.lk';

UPDATE source_endpoints AS endpoint
SET paused = false,
    health_state = 'unknown',
    last_error = NULL
FROM sources AS source
WHERE endpoint.source_id = source.id
  AND source.website = 'https://mawratanews.lk';

UPDATE rights_profiles AS rights
SET mode = 'discovery_only'
FROM sources AS source
WHERE rights.source_id = source.id
  AND source.website = 'https://mawratanews.lk';

COMMIT;
