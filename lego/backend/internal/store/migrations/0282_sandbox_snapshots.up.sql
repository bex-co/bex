-- w32/m285: Render's sandbox snapshots, a saved root filesystem a new sandbox
-- can start from (`render sandboxes snapshots create|list|get|delete`,
-- `render sandboxes create --from|--snapshot-id`; the pinned OpenAPI's
-- create-, list-, retrieve- and delete-sandbox-snapshot;
-- docs/ADR104-sandbox-render-parity.md D4).
--
-- A row is written when the capture is accepted (`creating`) and settles as
-- `available` or `failed`. The image is one repository of its own in the
-- sandbox registry, named by the row's id, so the registry's per-repository
-- tag retention can never evict it.
CREATE TABLE sandbox_snapshots (
    id                   text        PRIMARY KEY,
    workspace_id         text        NOT NULL,
    sandbox_group_id     text        NOT NULL,
    -- The sandbox the snapshot was captured from: its public id, recorded for
    -- reference as on Render, and its substrate id, which names the repository
    -- the commit Job pushed to.
    source_sandbox_id    text        NOT NULL,
    source_ref           text        NOT NULL,
    name                 text        CHECK (name IS NULL OR char_length(name) BETWEEN 1 AND 64),
    kind                 text        NOT NULL DEFAULT 'filesystem' CHECK (kind IN ('filesystem', 'runtime')),
    status               text        NOT NULL DEFAULT 'creating'
                                     CHECK (status IN ('creating', 'available', 'failed')),
    -- What a sandbox restored from the snapshot inherits unless its create
    -- overrides it: the source's plan, network policy (Render's object) and
    -- timeouts (0 means disabled, as on Render).
    plan                 text        NOT NULL,
    network_policy       jsonb       NOT NULL,
    timeout_seconds      integer     NOT NULL CHECK (timeout_seconds >= 0),
    idle_timeout_seconds integer     NOT NULL CHECK (idle_timeout_seconds >= 0),
    -- Render's ttlSeconds: the lifetime after the snapshot's last use; 0 keeps
    -- it until it is deleted, and expires_at is NULL then.
    ttl_seconds          bigint      NOT NULL CHECK (ttl_seconds >= 0),
    expires_at           timestamptz,
    -- Who asked for the capture, for the per-user capture rate limit.
    requested_by         text        NOT NULL DEFAULT '',
    requested_at         timestamptz NOT NULL DEFAULT now(),
    captured_at          timestamptz,
    -- The stored image's manifest digest and compressed size, once available.
    image_digest         text,
    size_bytes           bigint,
    error                text,
    -- The substrate's own id for the capture in flight (an OpenSandbox
    -- snapshot), cleared once the capture is settled and that object deleted.
    runtime_id           text,
    -- A bex-api replica settling the capture holds it until this time, so two
    -- replicas never copy the same image.
    claimed_until        timestamptz,
    -- A deleted snapshot leaves every read; the row stays so a repeated delete
    -- answers 204, as on Render. reaped_at is when its image left the registry.
    deleted_at           timestamptz,
    reaped_at            timestamptz,
    FOREIGN KEY (sandbox_group_id, workspace_id)
        REFERENCES sandbox_groups (id, workspace_id) ON DELETE CASCADE
);

-- The list is newest first and pages on the snapshot id; a restore by name
-- reads the newest available snapshot of that name.
CREATE INDEX sandbox_snapshots_newest ON sandbox_snapshots (sandbox_group_id, requested_at DESC, id DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX sandbox_snapshots_by_name ON sandbox_snapshots (sandbox_group_id, name, requested_at DESC)
    WHERE status = 'available' AND deleted_at IS NULL;
-- The settle sweep reads the captures in flight, and the limits count them.
CREATE INDEX sandbox_snapshots_creating ON sandbox_snapshots (workspace_id, requested_at)
    WHERE status = 'creating' AND deleted_at IS NULL;
-- The per-user capture rate limit counts a requester's last minute.
CREATE INDEX sandbox_snapshots_requested ON sandbox_snapshots (workspace_id, requested_by, requested_at DESC);
-- The reap sweep, every few seconds on each replica, reads the images still
-- to delete (deleted or expired) and the captures whose substrate object is
-- still to delete, through these two partial indexes rather than the table.
CREATE INDEX sandbox_snapshots_unreaped ON sandbox_snapshots (expires_at)
    WHERE image_digest IS NOT NULL AND reaped_at IS NULL;
CREATE INDEX sandbox_snapshots_captures ON sandbox_snapshots (requested_at)
    WHERE runtime_id IS NOT NULL;
