-- w2/m38 migration 0033 rollback.
DROP INDEX IF EXISTS deploys_app_updated_idx;
DROP INDEX IF EXISTS deploys_one_open_per_app_idx;
ALTER TABLE deploys DROP COLUMN IF EXISTS updated_at;
