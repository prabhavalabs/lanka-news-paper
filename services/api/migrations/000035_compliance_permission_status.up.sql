BEGIN;

ALTER TABLE source_compliance_reviews
  ADD CONSTRAINT source_compliance_permissions_require_review CHECK (
    status IN ('approved', 'restricted') OR NOT (
      allow_discovery OR allow_full_text_storage OR allow_ai_processing
      OR allow_embeddings OR allow_training OR allow_public_full_text
    )
  );

COMMIT;
