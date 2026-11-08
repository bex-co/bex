-- ADR101: workspace agent automations. An automation is a schedule trigger
-- plus a session template; triggers and template are validated by the service
-- layer and kept as jsonb. next_fire_at is the durable schedule cursor the
-- scheduler claims with FOR UPDATE SKIP LOCKED.
CREATE TABLE agent_automations (
    id                   text        PRIMARY KEY,
    workspace_id         text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name                 text        NOT NULL CHECK (name ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND length(name) <= 64),
    enabled              boolean     NOT NULL DEFAULT true,
    created_by           text        NOT NULL,
    triggers             jsonb       NOT NULL CHECK (jsonb_typeof(triggers) = 'array'),
    template             jsonb       NOT NULL CHECK (jsonb_typeof(template) = 'object'),
    overlap_policy       text        NOT NULL DEFAULT 'skip' CHECK (overlap_policy IN ('skip')),
    next_fire_at         timestamptz,
    last_fire_at         timestamptz,
    last_session_id      text,
    paused_reason        text        CHECK (paused_reason IS NULL
                                            OR paused_reason IN ('repeated_failures', 'creator_lost_access', 'template_invalid')),
    consecutive_failures integer     NOT NULL DEFAULT 0 CHECK (consecutive_failures >= 0),
    version              integer     NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, name)
);

CREATE INDEX agent_automations_workspace_created_idx
    ON agent_automations (workspace_id, created_at DESC, id DESC);

-- Serves the scheduler's claim query.
CREATE INDEX agent_automations_due_idx
    ON agent_automations (next_fire_at)
    WHERE enabled AND next_fire_at IS NOT NULL;

-- One row per fired tick. The primary key makes a tick fire at most once
-- across bex-api replicas; claimed_until is the lease a crashed claimer's
-- tick is recovered after.
CREATE TABLE agent_automation_runs (
    automation_id text        NOT NULL REFERENCES agent_automations (id) ON DELETE CASCADE,
    fire_at       timestamptz NOT NULL,
    outcome       text        NOT NULL CHECK (outcome IN ('claimed', 'started', 'skipped_overlap', 'failed_create', 'denied')),
    session_id    text,
    missed_ticks  integer     NOT NULL DEFAULT 0 CHECK (missed_ticks >= 0),
    error         text        NOT NULL DEFAULT '',
    claimed_until timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    settled_at    timestamptz,
    PRIMARY KEY (automation_id, fire_at)
);

CREATE INDEX agent_automation_runs_stale_idx
    ON agent_automation_runs (claimed_until)
    WHERE outcome = 'claimed';

-- The automation that started a session. No foreign key: deleting an
-- automation keeps the sessions it started.
ALTER TABLE agent_sessions ADD COLUMN automation_id text;

CREATE INDEX agent_sessions_automation_idx
    ON agent_sessions (workspace_id, automation_id)
    WHERE automation_id IS NOT NULL;
