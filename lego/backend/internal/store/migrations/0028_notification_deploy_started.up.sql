-- w3/005: complete the per-member deploy lifecycle preferences with the
-- request-time "deploy started" event. Existing rows opt in by default, matching
-- the notification feature's no-row default (all three lifecycle events true).
--
-- Repair the prerequisite as part of this migration. Production databases that
-- had already advanced past migration 0013 before notification_settings was
-- renumbered into that slot silently skipped its CREATE TABLE; a bare ALTER then
-- left version 28 dirty and blocked the whole bex-api rollout. IF NOT EXISTS
-- keeps the normal 0013 -> 0028 path byte-for-byte in shape while making the
-- skipped-0013 path converge to the same final schema.
CREATE TABLE IF NOT EXISTS notification_settings (
    id               text PRIMARY KEY,
    tenant_id        text NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    subject          text NOT NULL,
    deploy_succeeded boolean NOT NULL DEFAULT true,
    deploy_failed    boolean NOT NULL DEFAULT true,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS notification_settings_member_idx
    ON notification_settings (tenant_id, subject);

ALTER TABLE notification_settings
    ADD COLUMN IF NOT EXISTS deploy_started boolean NOT NULL DEFAULT true;
