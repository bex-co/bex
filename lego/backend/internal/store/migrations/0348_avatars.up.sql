-- w1/m648: an account's and a workspace's avatar, as on Render. Render keeps an
-- avatar as one uploaded image: a photo the dashboard crops to a 128x128 JPEG,
-- or a monogram the dashboard draws and uploads the same way
-- (docs/render-artifacts/avatars.md). So there is one row or none per owner,
-- and no monogram column: the default monogram is drawn by the client from the
-- email or the workspace name.
--
--   owner_id      own-<xid> for an ACCOUNT (the account's durable id, which
--                 its public usr- id spells), tea-<xid> for a WORKSPACE.
--   workspace_id  the workspace again for a WORKSPACE row, so the row goes
--                 with the workspace (ON DELETE CASCADE). An ACCOUNT row goes
--                 with the account: store.CleanupAccountSubject deletes it.
--   download_key  128 random bits, the path of the image's public URL
--                 (GET /v1/avatars/<key>). A new upload gets a new key, so a
--                 browser may cache an image for good.
--   content       the image's bytes, at most 1 MiB (the dashboard uploads a
--                 few kilobytes).
CREATE TABLE avatars (
    owner_id     text PRIMARY KEY,
    avatar_type  text NOT NULL CHECK (avatar_type IN ('ACCOUNT', 'WORKSPACE')),
    workspace_id text REFERENCES tenants (id) ON DELETE CASCADE,
    download_key text NOT NULL UNIQUE,
    content_type text NOT NULL CHECK (content_type IN ('image/jpeg', 'image/png')),
    content      bytea NOT NULL CHECK (octet_length(content) BETWEEN 1 AND 1048576),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT avatars_workspace_owner CHECK (
        (avatar_type = 'WORKSPACE' AND workspace_id = owner_id)
        OR (avatar_type = 'ACCOUNT' AND workspace_id IS NULL))
);

-- An upload slot avatarPresignedUpload mints: Render's presigned PUT, here a
-- bex-api URL whose token only this table knows (its SHA-256, never the
-- token). The PUT redeems it once, before expires_at, with exactly size_bytes
-- of body. subject is the person who asked; account deletion deletes their
-- open slots.
CREATE TABLE avatar_uploads (
    token_hash   bytea PRIMARY KEY,
    owner_id     text NOT NULL,
    avatar_type  text NOT NULL CHECK (avatar_type IN ('ACCOUNT', 'WORKSPACE')),
    workspace_id text REFERENCES tenants (id) ON DELETE CASCADE,
    size_bytes   integer NOT NULL CHECK (size_bytes BETWEEN 1 AND 1048576),
    subject      text NOT NULL,
    expires_at   timestamptz NOT NULL,
    CONSTRAINT avatar_uploads_workspace_owner CHECK (
        (avatar_type = 'WORKSPACE' AND workspace_id = owner_id)
        OR (avatar_type = 'ACCOUNT' AND workspace_id IS NULL))
);

CREATE INDEX avatar_uploads_expires_at ON avatar_uploads (expires_at);
