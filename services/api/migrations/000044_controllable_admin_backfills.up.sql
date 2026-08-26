BEGIN;

ALTER TABLE admin_analysis_backfills
  DROP CONSTRAINT admin_analysis_backfill_status_valid,
  ADD CONSTRAINT admin_analysis_backfill_status_valid
    CHECK (status IN (
      'queued', 'running', 'paused', 'completed', 'partially_completed', 'failed', 'cancelled'
    ));

ALTER TABLE admin_analysis_backfill_items
  DROP CONSTRAINT admin_analysis_backfill_item_state_valid,
  ADD CONSTRAINT admin_analysis_backfill_item_state_valid
    CHECK (state IN ('pending', 'queued', 'running', 'succeeded', 'failed', 'cancelled'));

CREATE TRIGGER queue_monitor_admin_backfills_changed
AFTER INSERT OR UPDATE OR DELETE ON admin_analysis_backfills
FOR EACH STATEMENT EXECUTE FUNCTION notify_queue_monitor_change();

CREATE TRIGGER queue_monitor_admin_backfill_items_changed
AFTER INSERT OR UPDATE OR DELETE ON admin_analysis_backfill_items
FOR EACH STATEMENT EXECUTE FUNCTION notify_queue_monitor_change();

COMMIT;
