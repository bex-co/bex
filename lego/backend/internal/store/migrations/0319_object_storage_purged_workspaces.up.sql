-- 0319: a deleted workspace whose object storage purge finished (ADR112 D9).
--
-- The bucket and usage rows are how the hourly sweep finds <tea-id>/, and they
-- are dropped once objectstorage.PurgeHold has passed. FinishObjectStoragePurge
-- writes this row in the same transaction, so the database keeps its own record
-- that it deleted the workspace and emptied the prefix, and the sweep looks
-- under that prefix again, a bounded number of workspaces each pass, for an
-- upload that arrived after the last purge. Only this row lets the sweep delete
-- under a prefix that has no other row: a prefix no row names is never evidence
-- of a deletion, since a workspace being created, or a bex-api holding the
-- credential with another database, has no row for it either.
--
--   workspace_id  the deleted workspace (no tenant foreign key: the tenant row
--                 is gone before this row is written, and tea- ids are never
--                 reused).
--   purged_at     when the purge that dropped the rows finished.
--   checked_at    when the sweep last looked under the prefix; NULL until it
--                 first does. The sweep takes the least recently checked first.
CREATE TABLE object_storage_purged_workspaces (
    workspace_id text        PRIMARY KEY,
    purged_at    timestamptz NOT NULL,
    checked_at   timestamptz
);

CREATE INDEX object_storage_purged_workspaces_check_idx
    ON object_storage_purged_workspaces (checked_at NULLS FIRST, workspace_id);
