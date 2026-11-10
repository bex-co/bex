-- w22/m311: Render sandbox groups. A group is the unit a sandbox is created in
-- and a concurrency limit applies to (render.com/docs/sandboxes-creating and
-- /docs/sandboxes-limits, read 2026-11-10; docs/ADR042-sandbox-cluster-substrate.md
-- § Sandbox groups).
--
-- A workspace has at most a configured number of groups (Render: 10; the cap is
-- enforced in the store's create transaction, not here), names are unique within
-- the workspace, and the first group created in a workspace is its default. The
-- default is created lazily, on the first sandbox create or first read of a
-- sandbox that has no group, so there is no per-workspace backfill: a sandbox
-- with no row in sandbox_group_members belongs to the workspace's default group.
CREATE TABLE sandbox_groups (
    id                text        PRIMARY KEY,
    workspace_id      text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name              text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    region            text        NOT NULL DEFAULT 'oregon',
    -- The configured limit on active sandboxes (at least 1). The effective limit
    -- the API reports is this value capped by the platform's maximum.
    concurrency_limit integer     NOT NULL CHECK (concurrency_limit >= 1),
    is_default        boolean     NOT NULL DEFAULT false,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT sandbox_groups_workspace_name_key UNIQUE (workspace_id, name),
    -- The target of sandbox_group_members' composite key, so a sandbox can never
    -- be bound to another workspace's group.
    CONSTRAINT sandbox_groups_id_workspace_key UNIQUE (id, workspace_id)
);

-- One default group per workspace, enforced by the database.
CREATE UNIQUE INDEX sandbox_groups_one_default ON sandbox_groups (workspace_id) WHERE is_default;

-- The list is newest first and pages on the group id.
CREATE INDEX sandbox_groups_newest ON sandbox_groups (workspace_id, created_at DESC, id DESC);

-- Which group a sandbox was created in, and the slot reservation that makes the
-- concurrency limit race-safe. The sandbox's status lives in OpenSandbox, not
-- here, so a row only binds the sandbox to its group; lease_until is a short
-- reservation written under the workspace's admission lock before a create or a
-- resume and read by the next admission's count, so a slot taken but not yet
-- visible in OpenSandbox's own list is still counted. Keyed by the public
-- sandbox id (sbx-…), pruned against the live OpenSandbox list like sandbox_names.
CREATE TABLE sandbox_group_members (
    workspace_id text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    sandbox_id   text        NOT NULL,
    group_id     text        NOT NULL,
    lease_until  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, sandbox_id),
    FOREIGN KEY (group_id, workspace_id) REFERENCES sandbox_groups (id, workspace_id)
);

CREATE INDEX sandbox_group_members_by_group ON sandbox_group_members (group_id);
