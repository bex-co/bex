-- w8/385: Render's deploy log narrates its port scan while a web service has
-- no open port ("==> No open ports detected, continuing to scan..."). The
-- control plane records when a rollout's stall first said no port was open,
-- and when a later stall said the scan was over (a port was found, or the
-- rollout moved on), so the log's scan lines keep their timestamps on every
-- read, after the deploy ends too. NULL is no scan.
ALTER TABLE deploys
  ADD COLUMN IF NOT EXISTS port_scan_since timestamptz,
  ADD COLUMN IF NOT EXISTS port_scan_until timestamptz;
