-- w5/154: workspace-scoped environment reads filter by tenant_id. They are
-- ListWorkspaceEnvironments, which serves the workspace scope index, and
-- countWorkspaceGroupings, the quota count every project or environment
-- create runs. No index led with tenant_id (0019 keys project_id and id), so
-- each read scanned every workspace's environments.
CREATE INDEX IF NOT EXISTS environments_tenant_id_idx
    ON environments (tenant_id);
