-- w4/701: repeat 0283 for every database whose recorded version had already
-- passed it. 0283_audit_source_ip_user_agent reached main after
-- 0284_cron_minimum, and production had recorded 284 by then, so golang-migrate
-- (which applies only versions above the recorded one) never ran 0283 there.
-- Every audit read and write then failed on the missing columns. A database
-- that did run 0283 (CI, every dev-N) sees a no-op here. 0283 keeps its number:
-- renaming it would re-run or orphan it on those databases.
--
-- scripts/migration_order_guard.py now refuses a push that adds a migration
-- numbered at or below the highest one on its base.
ALTER TABLE audit_events
  ADD COLUMN IF NOT EXISTS source_ip text,
  ADD COLUMN IF NOT EXISTS user_agent text;
