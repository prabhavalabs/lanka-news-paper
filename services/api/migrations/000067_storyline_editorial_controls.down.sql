BEGIN;
DROP TABLE storyline_event_exclusions;
ALTER TABLE storylines DROP COLUMN archived_by_admin;
ALTER TABLE storylines DROP COLUMN summary_locked;
COMMIT;
