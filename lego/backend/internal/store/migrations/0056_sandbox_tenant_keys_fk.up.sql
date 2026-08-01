-- Cascade sandbox tenant keys on workspace delete (w1/m61, ADR042 D4).
-- sandbox_tenant_keys (migration 0054) held workspace_id with no foreign key, so
-- a deleted workspace's OpenSandbox tenant key row survived forever and still
-- resolved through WorkspaceForSandboxKey — a live credential for a dead tenant.
-- Add the missing FK with ON DELETE CASCADE so the key row dies with its
-- workspace, exactly like every other workspace-keyed table.

-- First clear any orphaned key rows the pre-FK behavior already stranded (every
-- workspace deleted since w3/m32 shipped), or the constraint below cannot be
-- validated against existing data.
DELETE FROM sandbox_tenant_keys
WHERE workspace_id NOT IN (SELECT id FROM tenants);

ALTER TABLE sandbox_tenant_keys
    ADD CONSTRAINT sandbox_tenant_keys_workspace_fk
    FOREIGN KEY (workspace_id) REFERENCES tenants (id) ON DELETE CASCADE;
