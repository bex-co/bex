-- w8/m114: every non-secret fact about an environment group — identity,
-- owning workspace, name, Environment membership, service links, timestamps,
-- the key and file names its committed content holds, and the coordination
-- state of a content save. OpenBao keeps only the values: env-groups/<id>/env
-- and /files under the group's workspace tenant, plus an in-flight save's
-- op/<opId>/{old,new}-{env,files} recovery maps.
CREATE TABLE env_groups (
    id                 text        PRIMARY KEY,
    tenant_id          text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name               text        NOT NULL CHECK (name <> ''),
    environment_id     text        REFERENCES environments (id) ON DELETE SET NULL,
    -- Linked services' public ids in link order. No foreign key: a service's
    -- initial links are composed before its apps row exists, and a deleted
    -- service is unlinked by the App purger.
    service_links      text[]      NOT NULL DEFAULT '{}',
    -- Names only, never values: the list and get views show them without the
    -- sensitive scope, so they need not cost an OpenBao read.
    env_keys           text[]      NOT NULL DEFAULT '{}',
    file_names         text[]      NOT NULL DEFAULT '{}',
    created_at         timestamptz NOT NULL,
    updated_at         timestamptz NOT NULL,
    -- Compare-and-set token for the metadata columns above.
    meta_version       bigint      NOT NULL DEFAULT 1,
    -- A content save serializes on content_version: it claims the group busy
    -- (recording its operation below in the same statement), writes the two
    -- maps and their projection Secrets, then releases it idle with the next
    -- generation. content_generation is what revision tokens encode.
    content_state      text        NOT NULL DEFAULT 'idle'
        CHECK (content_state IN ('idle', 'busy', 'repair_required')),
    content_generation bigint      NOT NULL DEFAULT 1,
    content_version    bigint      NOT NULL DEFAULT 1,
    op_id              text,
    op_kind            text        CHECK (op_kind IN ('patch', 'clone')),
    op_phase           text        CHECK (op_phase IN ('admitted', 'env_written', 'files_written')),
    op_lease_until     timestamptz,
    op_env_changed     boolean     NOT NULL DEFAULT false,
    op_files_changed   boolean     NOT NULL DEFAULT false,
    CONSTRAINT env_groups_tenant_name_key UNIQUE (tenant_id, name),
    CONSTRAINT env_groups_op_complete CHECK (
        (op_id IS NULL AND op_kind IS NULL AND op_phase IS NULL AND op_lease_until IS NULL)
        OR (op_id IS NOT NULL AND op_kind IS NOT NULL AND op_phase IS NOT NULL AND op_lease_until IS NOT NULL)
    ),
    CONSTRAINT env_groups_busy_has_op CHECK (content_state <> 'busy' OR op_id IS NOT NULL)
);

CREATE INDEX env_groups_environment_idx ON env_groups (environment_id) WHERE environment_id IS NOT NULL;

-- Transitional (w8/m114 t008 drops it): bex-api serves environment groups only
-- once the one-shot cutover has copied the OpenBao metadata into env_groups and
-- written this row, so a new revision never answers from an empty table.
CREATE TABLE env_group_cutover (
    id           boolean     PRIMARY KEY DEFAULT true CHECK (id),
    completed_at timestamptz NOT NULL
);
