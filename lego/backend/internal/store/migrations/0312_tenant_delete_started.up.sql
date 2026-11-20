-- w9/121: a workspace delete revokes every member's OpenFGA tuple before its
-- pre-cascade purgers run, so that nobody can create a resource or mint a key
-- while the teardown is in flight. When a purger then fails, the row survives
-- for the retry but no one holds can_manage on it any more. delete_started_at
-- records that a delete got that far: it lets a subject this row's
-- tenant_members still records as an admin retry the delete, and refuses every
-- path that would grant a tuple on the workspace again. The cascade drops the
-- row with it; NULL means no delete has started.
ALTER TABLE tenants
    ADD COLUMN delete_started_at timestamptz;
