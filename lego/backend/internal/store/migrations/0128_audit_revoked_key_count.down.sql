-- Reverse w2/m163's audit detail column. Dropping it loses the record of how
-- many keys each past removal revoked; the removals themselves are unaffected.
ALTER TABLE audit_events DROP COLUMN IF EXISTS revoked_key_count;
