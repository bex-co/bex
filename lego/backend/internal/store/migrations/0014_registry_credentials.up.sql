-- w2/m14: a workspace's stored credentials for a private external image
-- registry (Docker Hub, GHCR, GitLab Container Registry, ECR, etc.), so an
-- existing-image service can pull from a non-public, non-Zot source.
--
-- The actual secret (password/token) is NOT a column here — it lives in
-- OpenBao (docs/ADR013-secrets.md), the same store the env-vars/secret-files
-- feature already uses for exactly this class of tenant credential. This
-- table is metadata only: which credential exists, which host/username it's
-- for, whose workspace it belongs to, and when it expires. That split makes
-- "secret is never returned by a read/list query" structural (t001's
-- acceptance bar) rather than a policy an adapter could get wrong — the
-- secret value simply isn't in the rows this table's queries ever touch.
-- name is a human display label (Render's registryCredential.name) —
-- defaults to host at creation when the caller doesn't supply one, so the
-- dashboard list always has something readable to show.
CREATE TABLE registry_credentials (
    id           text PRIMARY KEY,
    workspace_id text NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name         text NOT NULL,
    host         text NOT NULL,
    username     text NOT NULL,
    expires_at   timestamptz,
    created_by   text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- List reads newest-first per workspace; the operator-wiring lookup (t002,
-- "does this workspace have a credential for this image's host") narrows by
-- both columns, so a composite index serves both.
CREATE INDEX registry_credentials_workspace_host_idx ON registry_credentials (workspace_id, host);
