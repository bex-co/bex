-- w8/382 (docs/ADR049-render-yaml-parity.md § Env groups renamed outside the
-- file): a Blueprint remembers each env group it applied by id, under the name
-- its file declares it as, so a sync after a dashboard rename finds the same
-- group, renames it back and never mints a second copy linked beside it.
-- Unlike blueprint_resource_claims this record is not exclusive: two Blueprints
-- may declare the same group, as they could before. A deleted group drops its
-- rows, so the next sync recreates it by name, as Render recreates a deleted
-- Blueprint resource.
CREATE TABLE blueprint_env_group_claims (
    tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    blueprint_id TEXT NOT NULL REFERENCES blueprints (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    env_group_id TEXT NOT NULL REFERENCES env_groups (id) ON DELETE CASCADE,
    claimed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (blueprint_id, name)
);

CREATE INDEX blueprint_env_group_claims_env_group
ON blueprint_env_group_claims (env_group_id);
