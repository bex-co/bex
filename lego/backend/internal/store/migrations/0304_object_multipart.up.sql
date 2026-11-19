-- ADR112 D3 (w21/m498 t013): S3 multipart uploads. An upload is a row naming the bucket
-- and key it will become and the Wasabi upload id behind it; the id bex hands out is the
-- row's typed id (mpu-<xid>), never Wasabi's. Its parts are rows, so a part number sent
-- again replaces the earlier one and the upload's bytes are the sum of the rows.
--
-- The parts' bytes count against the workspace's quota until the upload completes or is
-- aborted (the quota gate reads them through OpenMultipartBytes), because the sweep's
-- object listing does not see them. last_part_at is the upload's creation until its first
-- part, then the latest part: the sweep aborts an upload with none for 7 days.
--
-- region is the bucket's, copied here so the quota read, the sweep and a workspace purge
-- reach the right Wasabi bucket without the bucket row, which a purge deletes first.
-- workspace_id and bucket_id have no foreign keys, like the other object_* tables: a
-- workspace delete aborts its uploads at Wasabi first and then drops the rows.
CREATE TABLE object_multipart_uploads (
    id               text        PRIMARY KEY,
    workspace_id     text        NOT NULL,
    bucket_id        text        NOT NULL,
    region           text        NOT NULL,
    key              text        NOT NULL CHECK (length(key) BETWEEN 1 AND 1024),
    wasabi_upload_id text        NOT NULL,
    created_by       text        NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    last_part_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX object_multipart_uploads_list_idx ON object_multipart_uploads (workspace_id, bucket_id, key, id);
CREATE INDEX object_multipart_uploads_stale_idx ON object_multipart_uploads (last_part_at);

CREATE TABLE object_multipart_parts (
    upload_id   text        NOT NULL REFERENCES object_multipart_uploads (id) ON DELETE CASCADE,
    part_number integer     NOT NULL CHECK (part_number BETWEEN 1 AND 10000),
    size        bigint      NOT NULL CHECK (size >= 0),
    etag        text        NOT NULL,
    uploaded_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (upload_id, part_number)
);
