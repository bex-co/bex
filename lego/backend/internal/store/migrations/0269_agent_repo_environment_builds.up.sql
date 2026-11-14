-- w9/m407, ADR111: repository environment snapshots (phase 2).
--
-- version is the SHA-256 of the setup steps' canonical JSON (ADR111 D7):
-- jsonb's own text form, whose key order and spacing Postgres fixes, so the
-- backfill here and every later write hash the same bytes. Checks are not
-- part of it: they change the overlay, not the image.
ALTER TABLE agent_repo_environments ADD COLUMN version text NOT NULL DEFAULT '';
UPDATE agent_repo_environments SET version = encode(sha256(convert_to(setup::text, 'UTF8')), 'hex');

-- A build bakes one repository's setup steps into an image the repository's
-- sessions boot from (ADR111 D1, D2). image_digest names the image at
-- snapshots/<ws>-sandbox/images/<id>; image_deleted_at marks an image retention
-- or an environment delete retired, after which no session boots from it. The
-- registry copy goes a day later, so a session that booted it just before
-- is not left without its image, and the digest is cleared once it has;
-- purge_after and purge_attempts back off a failed delete. base_image is the
-- agent template image the builder ran, so a session never boots a snapshot
-- of an older driver. steps is the redacted per-step log the driver reported.
-- size_bytes is reserved against the workspace's storage cap before the
-- image is copied.
CREATE TABLE agent_repo_environment_builds (
    id               text        PRIMARY KEY,
    workspace_id     text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    repo             text        NOT NULL CHECK (length(repo) BETWEEN 3 AND 255),
    version          text        NOT NULL,
    trigger          text        NOT NULL CHECK (trigger IN ('environment_saved', 'secret_changed', 'manual', 'refresh')),
    status           text        NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'building', 'success', 'failed', 'cancelled')),
    failure_code     text        CHECK (failure_code IN ('setup_failed', 'step_timeout', 'build_timeout', 'checkout_failed', 'storage_limit_reached', 'internal_error')),
    failure_reason   text,
    sandbox_id       text,
    base_image       text,
    image_digest     text,
    image_deleted_at timestamptz,
    purge_after      timestamptz,
    purge_attempts   integer     NOT NULL DEFAULT 0,
    size_bytes       bigint      CHECK (size_bytes >= 0),
    head_sha         text,
    steps            jsonb       NOT NULL DEFAULT '[]'::jsonb,
    created_at       timestamptz NOT NULL DEFAULT now(),
    started_at       timestamptz,
    finished_at      timestamptz,
    heartbeat_at     timestamptz
);

-- One queued and one running build per repository (ADR111 D3).
CREATE UNIQUE INDEX agent_repo_environment_builds_queued ON agent_repo_environment_builds (workspace_id, repo) WHERE status = 'queued';
CREATE UNIQUE INDEX agent_repo_environment_builds_building ON agent_repo_environment_builds (workspace_id, repo) WHERE status = 'building';
CREATE INDEX agent_repo_environment_builds_repo ON agent_repo_environment_builds (workspace_id, repo, created_at DESC);
CREATE INDEX agent_repo_environment_builds_claim ON agent_repo_environment_builds (created_at) WHERE status = 'queued';
CREATE INDEX agent_repo_environment_builds_images ON agent_repo_environment_builds (workspace_id) WHERE image_digest IS NOT NULL;
CREATE INDEX agent_repo_environment_builds_purge ON agent_repo_environment_builds (image_deleted_at) WHERE image_digest IS NOT NULL AND image_deleted_at IS NOT NULL;

-- The build a session's latest dispatch booted from (ADR111 D4); empty for
-- phase 1.
ALTER TABLE agent_sessions ADD COLUMN environment_build_id text NOT NULL DEFAULT '';
