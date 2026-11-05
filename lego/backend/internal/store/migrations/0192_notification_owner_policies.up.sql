-- w3/m271 t002: the workspace notification policy, Render's owner-level
-- notification settings (GET/PATCH /notification-settings/owners/{ownerId},
-- docs/ADR052-notifications.md D2). One row per workspace that changed it; a
-- workspace with no row has Render's default (email on, failure
-- notifications only), so existing workspaces need no backfill.
--
-- notifications_to_send is the level a service on "default" follows: 'none',
-- 'failure' or 'all'. Render's slackEnabled and previewNotificationsEnabled
-- are not stored: both are recorded non-goals and always read false.
CREATE TABLE notification_owner_policies (
    workspace_id          text        PRIMARY KEY REFERENCES tenants (id) ON DELETE CASCADE,
    email_enabled         boolean     NOT NULL DEFAULT true,
    notifications_to_send text        NOT NULL DEFAULT 'failure'
                                      CHECK (notifications_to_send IN ('none', 'failure', 'all')),
    updated_at            timestamptz NOT NULL DEFAULT now()
);
