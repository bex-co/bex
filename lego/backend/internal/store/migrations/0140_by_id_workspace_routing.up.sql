-- w4/m172: by-id verbs with no ownerId resolve the resource's OWN workspace
-- from its id before authorizing there. These indexes serve those unscoped
-- routing reads; both tables' existing keys lead with the workspace column,
-- so a lookup by id alone would otherwise scan.

-- GET /v1/events/{eventId}: ServiceEventWorkspaces reads workspace_id by
-- event_id. Not unique: an audit row filed under workspace:default is
-- attributed to every matching owner (migration 0083), so one evt-… id can be
-- indexed under more than one workspace.
CREATE INDEX IF NOT EXISTS service_event_index_event_id_idx
    ON service_event_index (event_id);

-- POST /v1/notifications/{id}/read: PushNotificationWorkspaces reads
-- tenant_id by the caller's (subject, event_id).
CREATE INDEX IF NOT EXISTS push_notifications_subject_event_idx
    ON push_notifications (subject, event_id);
