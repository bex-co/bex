-- ADR097 quota residual, w9/m180 t005: one service or build source create at
-- a time per workspace, held as a lease row rather than a Postgres lock, so a
-- create never holds a pooled connection while it waits or works. A build
-- source widens the namespace's App quota and then writes its App; a service
-- create in between could take that slot. The lease covers both writes.
-- A row whose expires_at has passed belongs to a crashed holder and may be
-- taken over.
CREATE TABLE workspace_create_leases (
    tenant_id  text PRIMARY KEY REFERENCES tenants (id) ON DELETE CASCADE,
    holder     text NOT NULL,
    expires_at timestamptz NOT NULL
);
