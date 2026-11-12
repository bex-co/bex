-- Reverse w4/m335's autoscaling target details. The autoscaling settings
-- themselves are unaffected; their events stop naming the targets.
ALTER TABLE audit_events
  DROP COLUMN IF EXISTS autoscaling_memory_to,
  DROP COLUMN IF EXISTS autoscaling_memory_from,
  DROP COLUMN IF EXISTS autoscaling_cpu_to,
  DROP COLUMN IF EXISTS autoscaling_cpu_from;
