BEGIN;

CREATE TABLE source_robots_cache (
  source_id uuid PRIMARY KEY REFERENCES sources(id) ON DELETE CASCADE,
  compliance_review_id uuid NOT NULL REFERENCES source_compliance_reviews(id) ON DELETE CASCADE,
  robots_url text NOT NULL,
  body_text text NOT NULL DEFAULT '',
  http_status integer NOT NULL,
  fetched_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  expires_at timestamptz NOT NULL,
  CONSTRAINT source_robots_cache_body_limit CHECK (octet_length(body_text) <= 1048576),
  CONSTRAINT source_robots_cache_status_valid CHECK (http_status BETWEEN 100 AND 599)
);

CREATE INDEX source_robots_cache_expiry ON source_robots_cache (expires_at);

COMMIT;
