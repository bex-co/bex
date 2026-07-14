-- 0019: named environments — a subset of a project's services (e.g.
-- staging/production), the second half of w1/m31's grouping feature. Follows
-- the exact same shape 0014_projects.up.sql set for projects: a workspace-
-- scoped label services opt-in to by having a nullable FK column set on
-- `apps`, not a join table.
--
-- An environment belongs to exactly one project; tenant_id is denormalized
-- from that project so environment-scoped queries never need a join to
-- authorize. Assigning a service to an environment (SetEnvironmentServices)
-- also stamps apps.project_id to the environment's project — Render's model
-- treats "in an environment" as "in that project" — so environment and
-- project membership can't independently drift for a service reached via
-- this path. (A service removed from its project via the separate
-- setProjectServices verb can still leave a stale apps.environment_id
-- pointing at an environment of a project it's no longer in; environment
-- reads defensively filter on both columns together, see
-- ListEnvironmentServices.)
CREATE TABLE environments (
    id          text        PRIMARY KEY,
    project_id  text        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    tenant_id   text        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        text        NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, name)
);

ALTER TABLE apps ADD COLUMN environment_id text REFERENCES environments(id) ON DELETE SET NULL;
