-- Normalize all public-schema table ownership to the application role.
--
-- Incident (2026-07-12): tenant_invites was owned by postgres because migration
-- 0004 was run manually as a superuser rather than through the migration runner
-- (which connects as the bex application user). The result was
-- "permission denied for table tenant_invites" on every invite-redemption call.
-- Fixed live with: ALTER TABLE tenant_invites OWNER TO bex
--
-- This migration makes the fix durable: any currently mis-owned table is
-- re-owned to the role running this migration (CURRENT_USER = bex when applied
-- via the standard BEX_CP_DB_URI connection string). Running as bex requires no
-- special privilege because bex already owns all these tables on a clean install;
-- running as a superuser also works (and is required if a table is mis-owned and
-- bex is not a superuser on that installation).
--
-- See also: store.CheckOwnership (startup guard) and docs/ADR003-control-plane.md.

ALTER TABLE tenants OWNER TO CURRENT_USER;
ALTER TABLE apps OWNER TO CURRENT_USER;
ALTER TABLE domains OWNER TO CURRENT_USER;
ALTER TABLE tenant_members OWNER TO CURRENT_USER;
ALTER TABLE tenant_invites OWNER TO CURRENT_USER;
ALTER TABLE deploys OWNER TO CURRENT_USER;
ALTER TABLE usage_hourly OWNER TO CURRENT_USER;
ALTER TABLE usage_monthly OWNER TO CURRENT_USER;
ALTER TABLE audit_events OWNER TO CURRENT_USER;
ALTER TABLE git_connections OWNER TO CURRENT_USER;
ALTER TABLE owner_ids OWNER TO CURRENT_USER;
