ALTER TABLE blueprint_syncs
  DROP COLUMN IF EXISTS error_notices,
  DROP COLUMN IF EXISTS commit_message;
