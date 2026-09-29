-- usage-coverage-diagnose.sql — which usage streams leave a workspace's coverage
-- short, and exactly where (w1/115). READ-ONLY: it only SELECTs.
--
--   psql "$DATABASE_URL" -v ws=tea-... [-v now=2026-09-29T07:30:00Z] \
--        -f scripts/usage-coverage-diagnose.sql
--
-- It mirrors PGStore.CurrentUsageCoverage + evaluateStreamCoverage
-- (lego/backend/internal/store/usage.go) hour by hour, instead of looking only
-- at the health rows that exist:
--
--   * the period is [month start, last closed hour) of `now` in UTC;
--   * a stream is in scope when expected_from < last closed hour and it is open
--     or its expected_through is after the month start;
--   * its expected hours run from max(hour(expected_from), month start) up to
--     min(last closed hour, hour(expected_through)), exclusive;
--   * its watermark (observed_through) stops at the first expected hour that has
--     NO health row or a non-healthy one; any non-healthy row in the period marks
--     the stream degraded even when it lies after an earlier gap.
--
-- Every in-scope stream is reported, including one with zero health rows (an
-- inner join or a WHERE on the health side would silently drop it), and an
-- absent hour is reported as missing evidence, which a MIN over the rows that
-- do exist can never find. Billable usage is reported separately: a missing
-- usage_hourly row is not missing health evidence, and neither implies the other.
\set ON_ERROR_STOP on
\if :{?ws}
\else
  \echo 'usage: psql -v ws=<workspace id> [-v now=<RFC3339>] -f scripts/usage-coverage-diagnose.sql'
  \quit
\endif
\if :{?now}
\else
  SELECT to_char(now() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS now \gset
\endif
BEGIN READ ONLY;
SET LOCAL TIME ZONE 'UTC';

WITH params AS (
  SELECT :'ws'::text AS ws,
         date_trunc('month', :'now'::timestamptz) AS month_start,
         date_trunc('hour', :'now'::timestamptz) AS latest_closed
), streams AS (
  SELECT s.resource_kind, s.service_id, s.kind, s.source, s.expected_from, s.expected_through,
         greatest(date_trunc('hour', s.expected_from), p.month_start) AS first_hour,
         least(p.latest_closed, coalesce(date_trunc('hour', s.expected_through), p.latest_closed)) AS target
  FROM usage_source_streams s CROSS JOIN params p
  WHERE s.workspace_id = p.ws
    AND s.expected_from < p.latest_closed
    AND (s.expected_through IS NULL OR s.expected_through > p.month_start)
), expected AS (
  SELECT st.resource_kind, st.service_id, st.kind, st.source, gs.hour
  FROM streams st
  CROSS JOIN LATERAL generate_series(st.first_hour, st.target - interval '1 hour', interval '1 hour') AS gs(hour)
), evidence AS (
  SELECT e.*, h.state,
         EXISTS (
           SELECT 1 FROM usage_hourly u, params p
           WHERE u.workspace_id = p.ws AND u.resource_kind = e.resource_kind
             AND u.service_id = e.service_id AND u.kind = e.kind AND u.window_start = e.hour
         ) AS has_usage
  FROM expected e
  LEFT JOIN usage_source_health h
    ON h.resource_kind = e.resource_kind AND h.service_id = e.service_id
   AND h.kind = e.kind AND h.source = e.source AND h.window_start = e.hour
), per_stream AS (
  SELECT resource_kind, service_id, kind, source,
         count(*) AS expected_hours,
         count(state) AS health_rows,
         min(hour) FILTER (WHERE state IS NULL) AS first_missing_health,
         min(hour) FILTER (WHERE state IS NOT NULL AND state <> 'healthy') AS first_unhealthy_in_range,
         count(*) FILTER (WHERE has_usage) AS usage_hours,
         min(hour) FILTER (WHERE NOT has_usage) AS first_missing_usage
  FROM evidence
  GROUP BY 1, 2, 3, 4
), degraded AS (
  -- evaluateStreamCoverage's health scan covers every row in the period, not
  -- only the expected range.
  SELECT h.resource_kind, h.service_id, h.kind, h.source, min(h.window_start) AS first_unhealthy_any
  FROM usage_source_health h CROSS JOIN params p
  WHERE h.workspace_id = p.ws
    AND h.window_start >= p.month_start AND h.window_start < p.latest_closed AND h.state <> 'healthy'
  GROUP BY 1, 2, 3, 4
)
SELECT st.resource_kind, st.service_id, st.kind, st.source,
       st.expected_from, st.expected_through, st.first_hour, st.target,
       coalesce(ps.expected_hours, 0) AS expected_hours,
       coalesce(ps.health_rows, 0) AS health_rows,
       ps.first_missing_health,
       coalesce(d.first_unhealthy_any, ps.first_unhealthy_in_range) AS first_unhealthy,
       least(ps.first_missing_health, ps.first_unhealthy_in_range, st.target) AS observed_through,
       CASE
         WHEN st.target <= st.first_hour THEN 'nothing expected yet'
         WHEN ps.first_missing_health IS NOT NULL
              AND ps.first_missing_health <= coalesce(ps.first_unhealthy_in_range, 'infinity')
           THEN 'behind: no health evidence at ' || to_char(ps.first_missing_health, 'YYYY-MM-DD"T"HH24"Z"')
         WHEN ps.first_unhealthy_in_range IS NOT NULL
           THEN 'behind: unhealthy at ' || to_char(ps.first_unhealthy_in_range, 'YYYY-MM-DD"T"HH24"Z"')
         WHEN d.first_unhealthy_any IS NOT NULL THEN 'caught up, but degraded'
         ELSE 'complete'
       END AS verdict,
       coalesce(ps.usage_hours, 0) AS usage_hours,
       ps.first_missing_usage
FROM streams st
LEFT JOIN per_stream ps USING (resource_kind, service_id, kind, source)
LEFT JOIN degraded d USING (resource_kind, service_id, kind, source)
ORDER BY observed_through NULLS FIRST, st.resource_kind, st.service_id, st.kind, st.source;

COMMIT;
