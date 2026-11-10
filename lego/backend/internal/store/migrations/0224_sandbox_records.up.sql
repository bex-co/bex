-- w3/m318: Render keeps a terminated sandbox readable (GET answers it with
-- status terminated and terminatedAt, list-sandboxes returns it under
-- status=terminated, a connect-token mint answers 409). OpenSandbox deletes the
-- object on terminate, so bex records each sandbox here: written at create and
-- stamped with its end by terminate, the lifetime reaper or the sweeper that
-- notices OpenSandbox stopped listing it. view is
-- the sandbox's public read shape as created (nothing about a sandbox changes
-- after it). A record ended more than 30 days ago is deleted
-- (SandboxRecordRetention).
CREATE TABLE sandbox_records (
    workspace_id  text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    sandbox_id    text        NOT NULL,
    owner         text        NOT NULL,
    view          jsonb       NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    terminated_at timestamptz,
    PRIMARY KEY (workspace_id, sandbox_id)
);

CREATE INDEX sandbox_records_ended_by_sandbox ON sandbox_records (sandbox_id) WHERE terminated_at IS NOT NULL;
CREATE INDEX sandbox_records_ended ON sandbox_records (terminated_at) WHERE terminated_at IS NOT NULL;
