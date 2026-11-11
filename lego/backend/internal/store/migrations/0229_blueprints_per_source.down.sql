-- Reverse 0229: one Blueprint per repo and branch again. Of each source's rows
-- keep one (a live row before a disconnected one, then the oldest) and delete
-- the rest, whose syncs, claims and intents cascade; then restore the key.
DROP INDEX blueprints_tenant_repo_branch;
DELETE FROM blueprints WHERE id IN (
    SELECT id FROM (
        SELECT id, row_number() OVER (
            PARTITION BY tenant_id, repo, branch
            ORDER BY (status = 'disconnected'), created_at, id) AS n
        FROM blueprints) ranked
    WHERE n > 1);
ALTER TABLE blueprints ADD CONSTRAINT blueprints_tenant_id_repo_branch_key UNIQUE (tenant_id, repo, branch);
