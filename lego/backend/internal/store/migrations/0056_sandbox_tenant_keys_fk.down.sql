-- Reverse w1/m61's sandbox tenant-key FK. The orphan-clearing DELETE in the up
-- migration is not reversible (those rows were dead credentials for deleted
-- workspaces); dropping the constraint restores the pre-m61 shape.
ALTER TABLE sandbox_tenant_keys
    DROP CONSTRAINT IF EXISTS sandbox_tenant_keys_workspace_fk;
