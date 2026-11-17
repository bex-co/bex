-- w9/m451: a shell session's StartShellEvent and EndShellEvent rows name the
-- service instance the session ran on, Render's shell metadata.instance
-- (render.com/docs/audit-logs: "service, instance, source_ip, and
-- user_agent"). The service is the row's target; the instance has no other
-- home. NULL on every other row and on rows recorded before this. Retained for
-- the audit retention window and purged with the row.
ALTER TABLE audit_events
  ADD COLUMN IF NOT EXISTS instance_id text;

-- A shell session row is not a service event: Render's service events list
-- has no shell entry, and the per-service feed's index trigger reads apps and
-- tenants, which the ssh-gateway's role (INSERT-only on audit_events,
-- ADR035 § Database blast radius) must never be granted. The trigger skips
-- the gateway's two shell verbs (core.AuditVerbStartShell/EndShell).
DROP TRIGGER IF EXISTS audit_events_service_event_index_insert ON audit_events;
CREATE TRIGGER audit_events_service_event_index_insert
AFTER INSERT ON audit_events
FOR EACH ROW
WHEN (NEW.verb NOT IN ('shell.StartShell', 'shell.EndShell'))
EXECUTE FUNCTION bex_index_audit_service_event();
