BEGIN;

UPDATE sources
SET active = true,
    description = CASE website
      WHEN 'https://lankasara.com/si' THEN 'Sinhala news and current-affairs publication.'
      WHEN 'https://www.yukthiya.lk' THEN 'Registered independent Sinhala news and analysis publication.'
    END
WHERE website IN ('https://lankasara.com/si', 'https://www.yukthiya.lk');

UPDATE source_endpoints AS endpoint
SET paused = false,
    health_state = 'unknown',
    last_error = NULL,
    consecutive_failures = 0,
    backoff_until = NULL
FROM sources AS source
WHERE endpoint.source_id = source.id
  AND source.website IN (
    'https://lankasara.com/si',
    'https://www.yukthiya.lk',
    'https://sinhala.adaderana.lk'
  );

COMMIT;
