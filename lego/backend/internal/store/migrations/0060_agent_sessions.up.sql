-- ADR047 D3 / w3/m39: durable cloud coding-agent session lifecycle.
-- Agent configuration remains structured JSON so adding a driver/model option
-- does not require a schema migration, while the target and lifecycle fields
-- stay first-class and queryable.
CREATE TABLE agent_sessions (
    id           text        PRIMARY KEY,
    workspace_id text        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    repo         text        NOT NULL,
    branch       text        NOT NULL,
    agent_config jsonb       NOT NULL DEFAULT '{}'::jsonb,
    sandbox_id   text        NOT NULL DEFAULT '',
    phase        text        NOT NULL DEFAULT 'creating'
                             CHECK (phase IN ('creating', 'running', 'resuming', 'failed', 'canceling', 'canceled')),
    status       text        NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    canceled_at  timestamptz
);

CREATE INDEX agent_sessions_workspace_created_idx
    ON agent_sessions (workspace_id, created_at DESC, id DESC);

CREATE UNIQUE INDEX agent_sessions_live_sandbox_idx
    ON agent_sessions (sandbox_id) WHERE sandbox_id <> '';
