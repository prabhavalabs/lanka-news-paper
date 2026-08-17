UPDATE sources
SET website = 'https://praja.lk'
WHERE website = 'https://www.praja.lk';

UPDATE source_endpoints
SET url = 'https://praja.lk/feed/',
    health_state = 'unknown',
    last_error = NULL,
    consecutive_failures = 0,
    backoff_until = NULL
WHERE url = 'https://www.praja.lk/feed/';
