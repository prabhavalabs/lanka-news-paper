BEGIN;

WITH ranked_categories AS (
  SELECT a.event_id, a.category_id,
         row_number() OVER (
           PARTITION BY a.event_id
           ORDER BY count(*) DESC, a.category_id
         ) AS rank
  FROM articles a
  WHERE a.event_id IS NOT NULL AND a.category_id IS NOT NULL
  GROUP BY a.event_id, a.category_id
)
UPDATE event_clusters ec
SET category_id = ranked_categories.category_id
FROM ranked_categories
WHERE ranked_categories.event_id = ec.id
  AND ranked_categories.rank = 1
  AND ec.category_id IS NULL;

COMMIT;
