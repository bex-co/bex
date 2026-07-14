-- 0014: named projects — group services within a workspace (w1/m31).
-- A project is a workspace-scoped label; services opt-in by having project_id set.
CREATE TABLE projects (
    id          text        PRIMARY KEY,
    tenant_id   text        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        text        NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

ALTER TABLE apps ADD COLUMN project_id text REFERENCES projects(id) ON DELETE SET NULL;
