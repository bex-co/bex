-- 0229: a workspace may hold several Blueprints for one repo and branch (w8/m153).
--
-- Render: "You can create multiple Blueprints from a single YAML file. Each
-- Blueprint creates and manages a completely independent set of resources."
-- 0016's UNIQUE (tenant_id, repo, branch) made the source the identity, so a
-- second create answered 409 BLUEPRINT_CONNECTION_CONFLICT and a create after a
-- delete revived the deleted row under its old id. A Blueprint is now identified
-- by its id alone; the columns stay indexed for the lookups that filter by
-- source (the deploy-time registration and the create-retry lookup).
ALTER TABLE blueprints DROP CONSTRAINT blueprints_tenant_id_repo_branch_key;
CREATE INDEX blueprints_tenant_repo_branch ON blueprints (tenant_id, repo, branch);
