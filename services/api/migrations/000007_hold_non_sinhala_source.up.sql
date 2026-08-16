BEGIN;

UPDATE sources
SET active = false,
    description = 'Registered publisher; held because its official feed currently publishes English headlines.'
WHERE website = 'https://mawratanews.lk';

UPDATE source_endpoints AS endpoint
SET paused = true,
    health_state = 'failed',
    last_error = 'Feed is not predominantly Sinhala'
FROM sources AS source
WHERE endpoint.source_id = source.id
  AND source.website = 'https://mawratanews.lk';

UPDATE rights_profiles AS rights
SET mode = 'disabled'
FROM sources AS source
WHERE rights.source_id = source.id
  AND source.website = 'https://mawratanews.lk';

COMMIT;
