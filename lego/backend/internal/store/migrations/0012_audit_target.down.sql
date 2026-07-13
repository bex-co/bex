DROP INDEX deploys_app_finished_idx;
DROP INDEX audit_events_target_at_idx;
ALTER TABLE audit_events DROP COLUMN target;
