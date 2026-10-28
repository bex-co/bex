-- w4/431: storage rows written before w4/427 carry no tier, so a Free
-- Postgres or Key Value still priced its included storage for the current
-- month (pricing exempts only tier = 'free'). Re-key each untiered storage
-- row from the same datastore's instance_seconds row for the same hour: that
-- row records the plan the datastore had in that window, so a datastore that
-- changed plan mid-month is tiered per hour. An hour with two instance tiers
-- (a mid-hour plan change) is ambiguous and stays untiered. Only rows from
-- the current billing month (October 2026) are touched; earlier months are
-- invoiced.
UPDATE usage_hourly s
SET tier = i.tier
FROM (
    SELECT resource_kind, service_id, window_start, min(tier) AS tier
    FROM usage_hourly
    WHERE kind = 'instance_seconds'
      AND tier <> ''
      AND window_start >= '2026-10-01T00:00:00Z'
    GROUP BY resource_kind, service_id, window_start
    HAVING count(*) = 1
) i
WHERE s.kind = 'storage_gb_seconds'
  AND s.tier = ''
  AND s.window_start >= '2026-10-01T00:00:00Z'
  AND i.resource_kind = s.resource_kind
  AND i.service_id = s.service_id
  AND i.window_start = s.window_start
  AND NOT EXISTS (
      SELECT 1 FROM usage_hourly d
      WHERE d.resource_kind = s.resource_kind
        AND d.service_id = s.service_id
        AND d.kind = s.kind
        AND d.tier = i.tier
        AND d.window_start = s.window_start
  );
