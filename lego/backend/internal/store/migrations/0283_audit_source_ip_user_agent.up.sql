-- w9/m450: every audit row a person's or an API key's own request caused says
-- where it came from, as Render's audit log does ("source_ip and user_agent
-- metadata for audit events when available", render.com/docs/audit-logs).
--
-- source_ip is the client address through bex's trusted proxies (the edge's
-- True-Client-IP, core.TrustedProxies.SourceIP); user_agent is the request's
-- User-Agent, valid UTF-8 and at most 512 bytes (core.AuditUserAgent). NULL on
-- rows no request of the actor caused (sweeps, webhooks, operator commands)
-- and on rows recorded before this. Both are retained for the audit retention
-- window and purged with the row (docs/ADR086-account-deletion.md).
ALTER TABLE audit_events
  ADD COLUMN IF NOT EXISTS source_ip text,
  ADD COLUMN IF NOT EXISTS user_agent text;
