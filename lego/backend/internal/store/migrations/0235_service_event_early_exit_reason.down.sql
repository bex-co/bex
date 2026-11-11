-- Intentionally retain the relaxed reason-code check on rollback: narrowing it
-- would reject retained server_failed facts that already carry early_exit.
-- Older binaries never write that code. No event rows are rewritten or deleted.
SELECT 1;
