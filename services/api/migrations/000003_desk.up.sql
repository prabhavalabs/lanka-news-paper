BEGIN;

ALTER TABLE sources
  ADD COLUMN IF NOT EXISTS description text NOT NULL DEFAULT '';

ALTER TABLE categories
  ADD COLUMN IF NOT EXISTS held boolean NOT NULL DEFAULT false;

ALTER TABLE articles
  ADD COLUMN IF NOT EXISTS original_headline text,
  ADD COLUMN IF NOT EXISTS author text,
  ADD COLUMN IF NOT EXISTS classify_confidence numeric,
  ADD COLUMN IF NOT EXISTS classify_model text,
  ADD COLUMN IF NOT EXISTS editorial_note text;

ALTER TABLE source_endpoints
  ADD COLUMN IF NOT EXISTS consecutive_failures integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS backoff_until timestamptz;

CREATE TABLE IF NOT EXISTS quarantine_payloads (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  endpoint_id uuid NOT NULL REFERENCES source_endpoints(id),
  reason text NOT NULL,
  sample text,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

UPDATE sources SET description = 'බීබීසී සිංහල ප්‍රවෘත්ති සේවය.' WHERE name = 'BBC News Sinhala' AND description = '';
UPDATE sources SET description = 'ඉන්ද්‍රජාල රූපවාහිනී ජාලයේ ප්‍රවෘත්ති සේවය.' WHERE name = 'ITN News' AND description = '';

COMMIT;
