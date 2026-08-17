UPDATE sources
SET website = 'https://www.praja.lk'
WHERE website = 'https://praja.lk';

UPDATE source_endpoints
SET url = 'https://www.praja.lk/feed/',
    health_state = 'unknown',
    last_error = NULL,
    consecutive_failures = 0,
    backoff_until = NULL
WHERE url = 'https://praja.lk/feed/';
