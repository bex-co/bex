-- w1/m658: a sandbox's lifecycle timeline, as Render's sandbox details drawer
-- reads it (`sandboxEvents`: SandboxLifecycleEvent with lifecycleType and
-- trigger). bex-api writes one row at each transition it makes or observes:
-- created and ready at create, suspended by the verb or the idle sweeper,
-- resuming by the verb or for a command (trigger CONNECTION), terminated by the
-- verb. A sandbox created before this table has none; its created and
-- terminated events are read from its record instead. user_email is the
-- person who started it, as Render's created event names its user.
CREATE TABLE sandbox_lifecycle_events (
    id             text        PRIMARY KEY,
    workspace_id   text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    sandbox_id     text        NOT NULL,
    lifecycle_type text        NOT NULL CHECK (lifecycle_type IN ('CREATED', 'READY', 'SUSPENDING', 'SUSPENDED', 'RESUMING', 'TERMINATED', 'ERRORED')),
    trigger        text        NOT NULL DEFAULT '',
    user_email     text        NOT NULL DEFAULT '',
    at             timestamptz NOT NULL
);

CREATE INDEX sandbox_lifecycle_events_by_sandbox ON sandbox_lifecycle_events (workspace_id, sandbox_id, at DESC, id DESC);
CREATE INDEX sandbox_lifecycle_events_at ON sandbox_lifecycle_events (at);
