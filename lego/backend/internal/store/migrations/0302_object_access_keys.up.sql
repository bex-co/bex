-- ADR112 D3 (w21/m498): S3 access keys. A key is a row naming the workspace it
-- reaches and the person who made it; its 40-character secret is NOT here, it
-- lives in OpenBao at object-storage/keys/<id> (SigV4 needs the secret itself
-- to verify a request, so a hash would not do). The id, s3k-<xid>, is the
-- access key id a client sends.
--
-- workspace_id and created_by have no foreign key: a workspace delete or a
-- member's removal disposes the rows through the service, which also deletes the
-- OpenBao secret (a cascade would leave the secret behind).
--
-- permission is the ceiling an S3 request is authorized at (read: list and get;
-- readwrite: also put, copy, delete, multipart, bucket create and delete, CORS
-- and lifecycle). No key ever holds can_manage, so none can publish a bucket.
-- expires_at is NULL for a persistent key (CI, rclone, Terraform) and at most 12
-- hours after created_at for an ephemeral one, which is what `bex s3` mints (the
-- service enforces the 12 hours; a CHECK against now() would trip on clock skew).
-- created_sign_in is how the creator was signed in, the stamp a workspace's
-- Google-login requirement reads for a key (ADR012 § 9b), as ssh_keys has.
CREATE TABLE object_access_keys (
    id              text        PRIMARY KEY,
    workspace_id    text        NOT NULL,
    created_by      text        NOT NULL,
    name            text        NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
    permission      text        NOT NULL CHECK (permission IN ('read', 'readwrite')),
    created_sign_in text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz,
    last_used_at    timestamptz,
    revoked_at      timestamptz,
    -- broken marks a key revoked because its OpenBao secret is gone (OpenBao
    -- restored from an older snapshot): listed as broken, never usable again.
    broken          boolean     NOT NULL DEFAULT false
);

-- The live-key counts of a workspace (persistent) and of a creator (ephemeral),
-- and the workspace's listing.
CREATE INDEX object_access_keys_workspace_idx ON object_access_keys (workspace_id, created_at, id);
CREATE INDEX object_access_keys_creator_idx ON object_access_keys (created_by, workspace_id);
-- What the sweep deletes seven days after a key ended.
CREATE INDEX object_access_keys_retired_idx ON object_access_keys (revoked_at) WHERE revoked_at IS NOT NULL;
CREATE INDEX object_access_keys_expiry_idx ON object_access_keys (expires_at) WHERE expires_at IS NOT NULL;
