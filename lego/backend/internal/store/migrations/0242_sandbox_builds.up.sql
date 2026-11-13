-- w2/m358: Render's sandbox builds, an OCI image imported into a sandbox group
-- so a sandbox can start from it (`render sandbox-groups images import`,
-- `render sandboxes create --image`; the pinned OpenAPI's create-, list- and
-- retrieve-sandbox-build; docs/ADR104-sandbox-render-parity.md § Images).
--
-- A row is created once the source reference has resolved to a digest, so
-- source_digest is always known. A request for a name and digest the group
-- already has (any status but failed) returns that row instead of a new one;
-- the partial unique index below makes that race-safe. The registry credential
-- a private import uses is never stored.
CREATE TABLE sandbox_builds (
    id                text        PRIMARY KEY,
    workspace_id      text        NOT NULL,
    sandbox_group_id  text        NOT NULL,
    name              text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 255),
    -- Render's source {repository, image, reference}: the registry host, the
    -- image path in it, and the tag or digest the caller named.
    source_repository text        NOT NULL,
    source_image      text        NOT NULL,
    source_reference  text        NOT NULL,
    source_digest     text        NOT NULL,
    status            text        NOT NULL DEFAULT 'pending'
                                  CHECK (status IN ('pending', 'running', 'succeeded', 'failed')),
    failure_reason    text,
    -- The stored image's manifest digest and compressed size, once succeeded.
    image_digest      text,
    size_bytes        bigint,
    created_at        timestamptz NOT NULL DEFAULT now(),
    -- Advanced while an import runs, so an import whose bex-api replica died
    -- can be told from a slow one and failed.
    updated_at        timestamptz NOT NULL DEFAULT now(),
    finished_at       timestamptz,
    FOREIGN KEY (sandbox_group_id, workspace_id)
        REFERENCES sandbox_groups (id, workspace_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX sandbox_builds_dedupe ON sandbox_builds (sandbox_group_id, name, source_digest)
    WHERE status <> 'failed';

-- The list is newest first and pages on the build id; name resolution reads
-- the newest succeeded build of a name.
CREATE INDEX sandbox_builds_newest ON sandbox_builds (sandbox_group_id, created_at DESC, id DESC);
CREATE INDEX sandbox_builds_by_name ON sandbox_builds (sandbox_group_id, name, created_at DESC)
    WHERE status = 'succeeded';
CREATE INDEX sandbox_builds_in_flight ON sandbox_builds (updated_at) WHERE status IN ('pending', 'running');
-- The storage cap sums a workspace's stored images on every suspend.
CREATE INDEX sandbox_builds_stored ON sandbox_builds (workspace_id) INCLUDE (size_bytes) WHERE status = 'succeeded';
