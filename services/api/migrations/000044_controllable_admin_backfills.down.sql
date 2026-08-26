BEGIN;

DROP TRIGGER IF EXISTS queue_monitor_admin_backfill_items_changed ON admin_analysis_backfill_items;
DROP TRIGGER IF EXISTS queue_monitor_admin_backfills_changed ON admin_analysis_backfills;

UPDATE admin_analysis_backfill_items
SET state = 'failed', error_detail = COALESCE(error_detail, 'Backfill control migration rolled back.'),
    finished_at = COALESCE(finished_at, clock_timestamp()), updated_at = clock_timestamp()
WHERE state = 'cancelled';

UPDATE admin_analysis_backfills
SET status = 'failed', error_detail = COALESCE(error_detail, 'Backfill control migration rolled back.'),
    finished_at = COALESCE(finished_at, clock_timestamp()), updated_at = clock_timestamp()
WHERE status IN ('paused', 'cancelled');

ALTER TABLE admin_analysis_backfill_items
  DROP CONSTRAINT admin_analysis_backfill_item_state_valid,
  ADD CONSTRAINT admin_analysis_backfill_item_state_valid
    CHECK (state IN ('pending', 'queued', 'running', 'succeeded', 'failed'));

ALTER TABLE admin_analysis_backfills
  DROP CONSTRAINT admin_analysis_backfill_status_valid,
  ADD CONSTRAINT admin_analysis_backfill_status_valid
    CHECK (status IN ('queued', 'running', 'completed', 'partially_completed', 'failed'));

COMMIT;
