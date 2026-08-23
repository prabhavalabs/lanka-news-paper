BEGIN;

CREATE TEMP TABLE migration_000038_sources AS
SELECT id
FROM sources
WHERE website IN (
  'https://aithiya.lk',
  'https://www.anidda.lk',
  'https://www.bbc.com/sinhala',
  'https://dasathalankanews.com',
  'https://www.divaina.lk',
  'https://www.gossiplankanews.com',
  'https://www.itnnews.lk',
  'https://www.infosrilanka.lk',
  'https://sinhala.lankanewsweb.net',
  'https://www.lankadeepa.lk',
  'https://medialk.com',
  'https://www.meepura.com',
  'https://www.news19.lk',
  'https://sinhala.newsfirst.lk',
  'https://praja.lk',
  'https://siyathanews.lk',
  'https://sinhala.srilankamirror.com',
  'https://www.vikalpa.org'
);

UPDATE source_collection_profiles
SET active = false
WHERE created_by = 'migration-000038'
  AND source_id IN (SELECT id FROM migration_000038_sources);

WITH previous AS (
  SELECT DISTINCT ON (profile.endpoint_id) profile.id
  FROM source_collection_profiles profile
  WHERE profile.source_id IN (SELECT id FROM migration_000038_sources)
    AND profile.created_by <> 'migration-000038'
  ORDER BY profile.endpoint_id, profile.version DESC
)
UPDATE source_collection_profiles profile
SET active = true
FROM previous
WHERE profile.id = previous.id;

UPDATE source_compliance_reviews
SET active = false
WHERE reviewed_by = 'operator:user-approved'
  AND source_id IN (SELECT id FROM migration_000038_sources);

WITH previous AS (
  SELECT DISTINCT ON (review.source_id) review.id
  FROM source_compliance_reviews review
  WHERE review.source_id IN (SELECT id FROM migration_000038_sources)
    AND review.reviewed_by <> 'operator:user-approved'
  ORDER BY review.source_id, review.version DESC
)
UPDATE source_compliance_reviews review
SET active = true
FROM previous
WHERE review.id = previous.id;

-- Move historical telemetry back to the restored policy versions before
-- removing the migration-created rows referenced by foreign keys.
WITH previous AS (
  SELECT DISTINCT ON (profile.endpoint_id)
         profile.endpoint_id,
         profile.id
  FROM source_collection_profiles profile
  WHERE profile.source_id IN (SELECT id FROM migration_000038_sources)
    AND profile.created_by <> 'migration-000038'
  ORDER BY profile.endpoint_id, profile.version DESC
)
UPDATE article_contents content
SET collection_profile_id = previous.id
FROM articles article
JOIN previous ON previous.endpoint_id = article.endpoint_id
WHERE content.article_id = article.id
  AND content.collection_profile_id IN (
    SELECT id FROM source_collection_profiles WHERE created_by = 'migration-000038'
  );

WITH previous AS (
  SELECT DISTINCT ON (profile.endpoint_id)
         profile.endpoint_id,
         profile.id
  FROM source_collection_profiles profile
  WHERE profile.source_id IN (SELECT id FROM migration_000038_sources)
    AND profile.created_by <> 'migration-000038'
  ORDER BY profile.endpoint_id, profile.version DESC
)
UPDATE crawl_attempts attempt
SET collection_profile_id = previous.id
FROM articles article
JOIN previous ON previous.endpoint_id = article.endpoint_id
WHERE attempt.article_id = article.id
  AND attempt.collection_profile_id IN (
    SELECT id FROM source_collection_profiles WHERE created_by = 'migration-000038'
  );

WITH previous AS (
  SELECT DISTINCT ON (review.source_id)
         review.source_id,
         review.id
  FROM source_compliance_reviews review
  WHERE review.source_id IN (SELECT id FROM migration_000038_sources)
    AND review.reviewed_by <> 'operator:user-approved'
  ORDER BY review.source_id, review.version DESC
)
UPDATE article_contents content
SET compliance_review_id = previous.id
FROM articles article
JOIN previous ON previous.source_id = article.source_id
WHERE content.article_id = article.id
  AND content.compliance_review_id IN (
    SELECT id FROM source_compliance_reviews WHERE reviewed_by = 'operator:user-approved'
  );

WITH previous AS (
  SELECT DISTINCT ON (rights.endpoint_id)
         rights.endpoint_id,
         rights.id
  FROM rights_profiles rights
  WHERE rights.source_id IN (SELECT id FROM migration_000038_sources)
    AND rights.approved_by IS DISTINCT FROM 'operator:user-approved-private-research'
  ORDER BY rights.endpoint_id, rights.version DESC
)
UPDATE articles article
SET rights_profile_id = previous.id
FROM previous
WHERE article.endpoint_id = previous.endpoint_id
  AND article.rights_profile_id IN (
    SELECT id
    FROM rights_profiles
    WHERE approved_by = 'operator:user-approved-private-research'
  );

DELETE FROM rights_profiles
WHERE approved_by = 'operator:user-approved-private-research'
  AND source_id IN (SELECT id FROM migration_000038_sources);

DELETE FROM source_collection_profiles
WHERE created_by = 'migration-000038'
  AND source_id IN (SELECT id FROM migration_000038_sources);

DELETE FROM source_compliance_reviews
WHERE reviewed_by = 'operator:user-approved'
  AND source_id IN (SELECT id FROM migration_000038_sources);

UPDATE source_endpoints endpoint
SET url = 'https://www.itnnews.lk/wp-json/wp/v2/posts?per_page=20&_fields=id,date,date_gmt,modified,modified_gmt,link,title,excerpt',
    polling_interval_seconds = 900
FROM sources source
WHERE endpoint.source_id = source.id
  AND source.website = 'https://www.itnnews.lk';

UPDATE source_endpoints endpoint
SET paused = true,
    polling_interval_seconds = 900,
    health_state = 'unknown'
FROM sources source
WHERE endpoint.source_id = source.id
  AND source.website IN (
    'https://www.gossiplankanews.com',
    'https://www.lankadeepa.lk'
  );

UPDATE source_endpoints endpoint
SET polling_interval_seconds = 900
FROM sources source
WHERE endpoint.source_id = source.id
  AND source.id IN (SELECT id FROM migration_000038_sources)
  AND source.website NOT IN (
    'https://www.bbc.com/sinhala',
    'https://www.gossiplankanews.com',
    'https://www.lankadeepa.lk'
  );

UPDATE sources
SET active = false,
    archived_at = clock_timestamp()
WHERE website = 'https://www.gossiplankanews.com';

COMMIT;
