-- 0016: blueprint registry — bex.yml stack sources (w2/m15).
-- A blueprint is a workspace-scoped repo+branch+manifest record created when
-- deploy is called with a repo; sync re-applies the stored manifest.
CREATE TABLE blueprints (
    id         text        PRIMARY KEY,
    tenant_id  text        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name       text        NOT NULL DEFAULT '',
    repo       text        NOT NULL DEFAULT '',
    branch     text        NOT NULL DEFAULT 'main',
    manifest   text        NOT NULL DEFAULT '',
    status     text        NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, repo, branch)
);
CREATE INDEX blueprints_tenant_id ON blueprints (tenant_id);
