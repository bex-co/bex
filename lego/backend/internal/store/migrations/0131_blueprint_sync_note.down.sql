-- Reverse w4/m125's sync-run annotation. Dropping it loses only the prose
-- explaining a takeover; no sync state or blueprint row depends on it.
ALTER TABLE blueprint_syncs DROP COLUMN IF EXISTS note;
