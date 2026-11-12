-- w4/597: an enable after a disable names the settings that were kept.
--
-- A disable now keeps the autoscaling settings, turned off, as Render does,
-- so the side before an enable can carry a range and targets that were not
-- running. This records whether they were: true or false where that side
-- had settings, NULL where it had none and on rows recorded before this,
-- whose settings were always the running ones.
ALTER TABLE audit_events
  ADD COLUMN IF NOT EXISTS autoscaling_enabled_from boolean;
