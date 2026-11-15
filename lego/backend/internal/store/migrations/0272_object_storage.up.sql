-- ADR112: object storage. A bucket is a row; every bucket of every workspace
-- shares one Wasabi bucket per region, and an object's key is
-- <workspace id>/<bucket id>/<key>. The key embeds the bucket's id, never its
-- name, so a bucket deleted and recreated under the same name never sees the
-- old objects.
--
-- workspace_id deliberately has no tenant foreign key (like
-- agent_session_attachments): deleting a workspace retires its bucket rows
-- (deleted_at) in the same transaction and the sweep purges the objects before
-- the rows go, instead of cascading them away and orphaning the objects.
--
-- system is a workspace's render-<region> bucket, the namespace Render's
-- /v1/objects routes read and write; it is created on first use and does not
-- count against the per-workspace bucket cap.
CREATE TABLE object_buckets (
    id                 text        PRIMARY KEY,
    workspace_id       text        NOT NULL,
    name               text        NOT NULL CHECK (length(name) BETWEEN 3 AND 63),
    region             text        NOT NULL CHECK (region <> ''),
    location_hint      text        NOT NULL DEFAULT '',
    system             boolean     NOT NULL DEFAULT false,
    public             boolean     NOT NULL DEFAULT false,
    cors               jsonb       NOT NULL DEFAULT '[]'::jsonb,
    lifecycle          jsonb       NOT NULL DEFAULT '[]'::jsonb,
    created_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz,
    purge_attempted_at timestamptz
);

-- A name is unique among a workspace's live buckets; a retired row waiting for
-- its purge does not hold the name.
CREATE UNIQUE INDEX object_buckets_live_name ON object_buckets (workspace_id, name) WHERE deleted_at IS NULL;
CREATE INDEX object_buckets_workspace_idx ON object_buckets (workspace_id, created_at, id) WHERE deleted_at IS NULL;
CREATE INDEX object_buckets_purge_idx
    ON object_buckets (purge_attempted_at NULLS FIRST, created_at) WHERE deleted_at IS NOT NULL;

-- What the hourly sweep last saw in each bucket's prefix. bucket_id has no
-- foreign key so the sweep can also record objects under an id with no row.
CREATE TABLE object_storage_usage (
    bucket_id    text        PRIMARY KEY,
    workspace_id text        NOT NULL,
    region       text        NOT NULL,
    bytes        bigint      NOT NULL CHECK (bytes >= 0),
    objects      bigint      NOT NULL CHECK (objects >= 0),
    observed_at  timestamptz NOT NULL
);

CREATE INDEX object_storage_usage_workspace_idx ON object_storage_usage (workspace_id, region);
