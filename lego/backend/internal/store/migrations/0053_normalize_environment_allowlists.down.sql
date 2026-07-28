-- The previous application version already accepts structured entries. A
-- rollback only needs to remove the strict storage guard; it must not discard
-- descriptions by converting canonical objects back to strings.
ALTER TABLE environments
    DROP CONSTRAINT IF EXISTS environments_ip_allow_list_structured;
