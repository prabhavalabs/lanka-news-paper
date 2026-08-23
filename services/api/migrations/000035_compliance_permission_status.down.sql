BEGIN;

ALTER TABLE source_compliance_reviews
  DROP CONSTRAINT IF EXISTS source_compliance_permissions_require_review;

COMMIT;
