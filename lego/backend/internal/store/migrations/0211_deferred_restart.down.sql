DROP INDEX IF EXISTS deploys_restart_deferred_idx;
ALTER TABLE deploys DROP COLUMN IF EXISTS restart_deferred;
