BEGIN;
ALTER TABLE storylines ADD COLUMN summary_locked boolean NOT NULL DEFAULT false;
ALTER TABLE storylines ADD COLUMN archived_by_admin boolean NOT NULL DEFAULT false;
CREATE TABLE storyline_event_exclusions (
  event_id uuid PRIMARY KEY REFERENCES event_clusters(id) ON DELETE CASCADE,
  storyline_id uuid NOT NULL REFERENCES storylines(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now()
);
COMMIT;
