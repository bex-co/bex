-- Reverse w4/563's env-group link details. The links themselves are
-- unaffected; their events stop naming the group.
ALTER TABLE audit_events
  DROP COLUMN IF EXISTS env_group_name,
  DROP COLUMN IF EXISTS env_group_id;
