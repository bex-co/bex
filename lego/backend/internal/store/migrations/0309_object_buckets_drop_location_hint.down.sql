-- Re-adds the column for a rollback to a binary that still reads it. Rows keep
-- no hint (the up migration discarded it); the empty-string default is what
-- such a binary treats as "created without a hint".
ALTER TABLE object_buckets
    ADD COLUMN location_hint text NOT NULL DEFAULT '';
