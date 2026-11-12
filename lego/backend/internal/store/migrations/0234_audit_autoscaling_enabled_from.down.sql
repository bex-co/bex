-- Reverse w4/597's record of whether the settings before an autoscaling
-- change were running. The settings themselves are unaffected.
ALTER TABLE audit_events
  DROP COLUMN IF EXISTS autoscaling_enabled_from;
