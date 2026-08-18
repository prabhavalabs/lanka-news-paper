BEGIN;

UPDATE sources
SET active = false,
    description = CASE website
      WHEN 'https://lankasara.com/si' THEN 'Official feed reachable; held because it has published no new Sinhala article since 2026-07-31.'
      WHEN 'https://www.yukthiya.lk' THEN 'Official API reachable; held because it has published no titled Sinhala article since 2026-07-23.'
    END
WHERE website IN ('https://lankasara.com/si', 'https://www.yukthiya.lk');

UPDATE source_endpoints AS endpoint
SET paused = true,
    health_state = 'stale',
    last_error = NULL,
    backoff_until = NULL
FROM sources AS source
WHERE endpoint.source_id = source.id
  AND source.website IN ('https://lankasara.com/si', 'https://www.yukthiya.lk');

UPDATE source_endpoints AS endpoint
SET health_state = 'unknown',
    last_error = NULL,
    consecutive_failures = 0,
    backoff_until = NULL
FROM sources AS source
WHERE endpoint.source_id = source.id
  AND source.website = 'https://sinhala.adaderana.lk';

COMMIT;
