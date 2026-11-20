-- w2/m532: Render-compatible sandbox group credential metadata & retained
-- version history (docs/ADR122-sandbox-credential-brokering.md).
--
-- Secret values are NEVER stored in Postgres; they live write-only in OpenBao
-- under tenants/<workspace_id>/sandbox-groups/<sandbox_group_id>/credentials/
-- <sha256(credential_id)>/<secret_ref>. Postgres holds configuration metadata,
-- monotonic version history (retaining current + 3 most recently active
-- distinct prior configurations), group-salted SHA-256 fingerprints, and
-- serialized byte counts so the 25-credential / 200 KiB / 1 MiB group caps are
-- enforced atomically under the sandbox_groups row lock.

ALTER TABLE sandbox_groups
    ADD COLUMN credential_fingerprint_salt bytea;

CREATE TABLE sandbox_group_credentials (
    workspace_id             text        NOT NULL,
    sandbox_group_id         text        NOT NULL,
    credential_id            text        NOT NULL,
    current_version_id       bigint      NOT NULL CHECK (current_version_id >= 1),
    next_version_id          bigint      NOT NULL CHECK (next_version_id >= 2),
    domain                   text        NOT NULL,
    enabled                  boolean     NOT NULL DEFAULT true,
    methods                  text[]      NOT NULL DEFAULT '{}',
    paths                    text[]      NOT NULL DEFAULT '{}',
    injection_type           text        NOT NULL CHECK (injection_type IN ('header', 'basic', 'cookie', 'query')),
    injection_name           text        NOT NULL DEFAULT '',
    injection_template       text        NOT NULL DEFAULT '',
    current_fingerprint      text        NOT NULL CHECK (char_length(current_fingerprint) = 64),
    current_secret_ref       text        NOT NULL,
    current_serialized_bytes integer     NOT NULL CHECK (current_serialized_bytes >= 0),
    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (sandbox_group_id, credential_id),
    FOREIGN KEY (sandbox_group_id, workspace_id)
        REFERENCES sandbox_groups (id, workspace_id) ON DELETE CASCADE
);

CREATE INDEX sandbox_group_credentials_workspace_idx
    ON sandbox_group_credentials (workspace_id, sandbox_group_id, credential_id);

CREATE TABLE sandbox_group_credential_versions (
    workspace_id             text        NOT NULL,
    sandbox_group_id         text        NOT NULL,
    credential_id            text        NOT NULL,
    version_id               bigint      NOT NULL CHECK (version_id >= 1),
    updated_at               timestamptz NOT NULL DEFAULT now(),
    updated_by               text        NOT NULL DEFAULT '',
    fingerprint              text        NOT NULL CHECK (char_length(fingerprint) = 64),
    restored_from_version_id bigint      CHECK (restored_from_version_id IS NULL OR restored_from_version_id >= 1),
    secret_ref               text        NOT NULL,
    config_json              jsonb       NOT NULL,
    secret_bytes             integer     NOT NULL CHECK (secret_bytes >= 0),
    config_serialized_bytes  integer     NOT NULL CHECK (config_serialized_bytes >= 0),
    version_meta_bytes       integer     NOT NULL CHECK (version_meta_bytes >= 0),
    PRIMARY KEY (sandbox_group_id, credential_id, version_id),
    FOREIGN KEY (sandbox_group_id, credential_id)
        REFERENCES sandbox_group_credentials (sandbox_group_id, credential_id) ON DELETE CASCADE,
    FOREIGN KEY (sandbox_group_id, workspace_id)
        REFERENCES sandbox_groups (id, workspace_id) ON DELETE CASCADE
);

CREATE INDEX sandbox_group_credential_versions_newest_idx
    ON sandbox_group_credential_versions (sandbox_group_id, credential_id, version_id DESC);
