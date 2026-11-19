-- ADR112 D8 (w21/m498 t010): the S3 surface's write-through counter. The quota gate
-- adds what bex itself has written since the sweep's last observation of a
-- workspace's region (completed puts, copies and multipart completions, from every
-- replica) to that observation, so a workspace cannot overshoot its cap for the up to
-- 72 hours a large region goes between listings.
--
-- One aggregate row per workspace and region; updated_at is the last write it
-- counts. The sweep's observation at time T deletes the row when updated_at <= T:
-- everything in it was written before the listing started and is in the listing. A
-- write that landed after T keeps the row, so those bytes are counted twice until the
-- next observation: the safe side.
--
-- workspace_id has no tenant foreign key, like the other object_* tables: a workspace
-- delete's purge drops the rows with its usage rows (FinishObjectStoragePurge).
CREATE TABLE object_storage_writes (
    workspace_id text        NOT NULL,
    region       text        NOT NULL,
    bytes        bigint      NOT NULL DEFAULT 0 CHECK (bytes >= 0),
    objects      bigint      NOT NULL DEFAULT 0 CHECK (objects >= 0),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, region)
);
