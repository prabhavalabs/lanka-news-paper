BEGIN;

DROP TABLE IF EXISTS quarantine_payloads;
ALTER TABLE source_endpoints DROP COLUMN IF EXISTS backoff_until;
ALTER TABLE source_endpoints DROP COLUMN IF EXISTS consecutive_failures;
ALTER TABLE articles DROP COLUMN IF EXISTS editorial_note;
ALTER TABLE articles DROP COLUMN IF EXISTS classify_model;
ALTER TABLE articles DROP COLUMN IF EXISTS classify_confidence;
ALTER TABLE articles DROP COLUMN IF EXISTS author;
ALTER TABLE articles DROP COLUMN IF EXISTS original_headline;
ALTER TABLE categories DROP COLUMN IF EXISTS held;
ALTER TABLE sources DROP COLUMN IF EXISTS description;

COMMIT;
