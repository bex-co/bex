-- Reverse w9/m451: index every audit row again, and drop the instance a shell
-- session row names.
DROP TRIGGER IF EXISTS audit_events_service_event_index_insert ON audit_events;
CREATE TRIGGER audit_events_service_event_index_insert
AFTER INSERT ON audit_events
FOR EACH ROW EXECUTE FUNCTION bex_index_audit_service_event();

ALTER TABLE audit_events
  DROP COLUMN IF EXISTS instance_id;
