-- w22/m313 (ADR104 D8, ADR023 § Sandbox snapshot storage): the objects the
-- sandbox_snapshot_gb_seconds meter accrues from.
--
-- One row per stored object: a suspended sandbox's retained rootfs image
-- (kind 'rootfs', object_id the sandbox's canonical id) or, once w32/m285
-- lands, a snapshot (kind 'snapshot', object_id the snapshot id). The row is
-- the durable cursor: started_at is when the object was first seen retained,
-- accrued_through how far its seconds have been added to usage_hourly,
-- ended_at when it stopped being retained (NULL while it is). bytes is NULL
-- until the image's size has been read from the registry: accrual waits for it
-- and then covers the whole retained interval, so a registry outage delays a
-- charge and neither loses nor invents seconds.
--
-- Timestamps are whole seconds, so byte-seconds fit a bigint; the sub-unit
-- remainder is carried in byte-seconds (decimal GB = 1e9 bytes).
CREATE TABLE sandbox_storage_objects (
    workspace_id    text        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    object_id       text        NOT NULL,
    kind            text        NOT NULL CHECK (kind IN ('rootfs', 'snapshot')),
    -- Where the registry holds it: the substrate sandbox id for a rootfs image.
    ref             text        NOT NULL DEFAULT '',
    bytes           bigint      CHECK (bytes IS NULL OR (bytes >= 0 AND bytes <= 17592186044416)),
    started_at      timestamptz NOT NULL,
    accrued_through timestamptz NOT NULL,
    remainder       bigint      NOT NULL DEFAULT 0 CHECK (remainder >= 0 AND remainder < 1000000000),
    ended_at        timestamptz,
    -- Set when the object ended because its sandbox is gone (terminated, or
    -- expired at pause + 30 days): the registry still holds the image, retention
    -- keeps a repository's five newest tags forever, and the reaper deletes it.
    -- A resume does not set it: the pod may still be pulling the image.
    reap_pending    boolean     NOT NULL DEFAULT false,
    -- When the sizer last tried this object: the unsized queue rotates on it, so
    -- an image whose size cannot be read does not starve newer ones.
    size_attempted_at timestamptz,
    PRIMARY KEY (workspace_id, object_id)
);

-- Every read the meter makes is served by a partial index over the objects
-- with work left, so its cost follows them and not the table's history.
CREATE INDEX sandbox_storage_objects_accrual_idx
    ON sandbox_storage_objects (accrued_through)
    WHERE bytes IS NOT NULL AND (ended_at IS NULL OR accrued_through < ended_at);
-- The unsized queue, in rotation order.
CREATE INDEX sandbox_storage_objects_unsized_idx
    ON sandbox_storage_objects (size_attempted_at NULLS FIRST, started_at)
    WHERE bytes IS NULL AND ended_at IS NULL AND kind = 'rootfs';
-- Closed objects that were never sized (the unsized_closed gauge, and pruning).
CREATE INDEX sandbox_storage_objects_unsized_closed_idx
    ON sandbox_storage_objects (ended_at)
    WHERE bytes IS NULL AND ended_at IS NOT NULL;
-- A workspace's open bytes, for the suspend-path cap read, as an index-only scan.
CREATE INDEX sandbox_storage_objects_open_idx
    ON sandbox_storage_objects (workspace_id) INCLUDE (bytes)
    WHERE ended_at IS NULL;
CREATE INDEX sandbox_storage_objects_reap_idx
    ON sandbox_storage_objects (ended_at)
    WHERE reap_pending;
-- Pruning: closed, settled and not waiting for the reaper.
CREATE INDEX sandbox_storage_objects_prune_idx
    ON sandbox_storage_objects (ended_at)
    WHERE ended_at IS NOT NULL AND NOT reap_pending;
