-- w4/m335: an autoscaling change records its CPU and memory targets.
--
-- The rows kept only the minimum and maximum, so a change that moved a target
-- alone read "1–3 instances → 1–3 instances". Render's
-- autoscaling_config_changed details carry criteria.cpu and criteria.memory on
-- both sides. One typed column per value, as the other audit details: the
-- target percentage before and after; NULL when that side was disabled or
-- had no target of that kind, for every other verb and for older rows.
ALTER TABLE audit_events
  ADD COLUMN IF NOT EXISTS autoscaling_cpu_from integer,
  ADD COLUMN IF NOT EXISTS autoscaling_cpu_to integer,
  ADD COLUMN IF NOT EXISTS autoscaling_memory_from integer,
  ADD COLUMN IF NOT EXISTS autoscaling_memory_to integer;
