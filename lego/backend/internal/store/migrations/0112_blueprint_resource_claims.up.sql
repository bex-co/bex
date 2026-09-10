-- w8/m40: workspace-scoped Blueprint resource claims. Coordinates replicas so
-- two Blueprints cannot both adopt the same App/Postgres/Key Value name; the
-- CR label bex.co/blueprint-id mirrors the durable row but is not the race
-- boundary. UNIQUE(tenant_id, kind, name) is the claim; takeover rewrites the
-- owner only when the confirmed previous owner still holds the row.
CREATE TABLE blueprint_resource_claims (
    tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    blueprint_id TEXT NOT NULL REFERENCES blueprints (id) ON DELETE CASCADE,
    claimed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, kind, name),
    CONSTRAINT blueprint_resource_claims_kind_chk
    CHECK (kind IN ('service', 'database', 'key_value'))
);

CREATE INDEX blueprint_resource_claims_blueprint
ON blueprint_resource_claims (blueprint_id);
