-- w8/m37 rollback: drop the execution-fencing state. Runs recorded while the
-- columns existed keep their rows; only the fencing metadata is removed.
DROP INDEX IF EXISTS blueprint_syncs_recovery_idx;
ALTER TABLE blueprint_syncs DROP COLUMN IF EXISTS execution_generation;
ALTER TABLE blueprints DROP COLUMN IF EXISTS active_run_id;
ALTER TABLE blueprints DROP COLUMN IF EXISTS execution_generation;
