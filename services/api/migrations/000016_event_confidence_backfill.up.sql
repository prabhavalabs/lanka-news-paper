BEGIN;

UPDATE event_clusters ec
SET confidence = scores.confidence
FROM (
  SELECT event_id, avg(clustering_score) confidence
  FROM event_articles
  GROUP BY event_id
) scores
WHERE scores.event_id = ec.id
  AND ec.confidence IS NULL;

COMMIT;
