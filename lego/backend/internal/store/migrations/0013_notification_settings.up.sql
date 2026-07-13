-- w3/m9: deploy notifications. Each row is one member's override of their
-- per-workspace deploy-email preferences (Render's /notification-settings).
-- A member with no row gets the default (both true, deploy_failed/succeeded
-- notify) — internal/notifications.Service applies that default at read time,
-- so absence means "default", not "opted out".
--
--   id                typed opaque id, ntf-<xid> (internal/id).
--   tenant_id/subject  the workspace + member the preference belongs to (subject
--                      is the OpenFGA user, same identity tenant_members keys on).
--   deploy_succeeded   email when a deploy of one of the caller's services goes live.
--   deploy_failed      email when a deploy fails (health-gate timeout or the App
--                      CR reaching Failed).
--
-- One row per (tenant, subject): a member customizes their own preferences only.
CREATE TABLE notification_settings (
    id               text PRIMARY KEY,
    tenant_id        text NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    subject          text NOT NULL,
    deploy_succeeded boolean NOT NULL DEFAULT true,
    deploy_failed    boolean NOT NULL DEFAULT true,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX notification_settings_member_idx ON notification_settings (tenant_id, subject);
