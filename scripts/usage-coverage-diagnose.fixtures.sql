-- Local fixtures for scripts/usage-coverage-diagnose.sql (w1/115). Load into a
-- throwaway database with the real migrations applied, then run the script
-- with -v ws=tea-x -v now=2026-09-01T06:30:00Z. Expected verdicts: srv-zero and
-- srv-gap behind at 00Z (no rows / missing first hour), srv-late behind at 01Z
-- and degraded at 03Z, srv-closed complete through 03Z, srv-ok complete with
-- usage; srv-foreign (another workspace) absent.
INSERT INTO tenants (id, name) VALUES ('tea-x', 'x'), ('tea-other', 'other');
-- now = 2026-09-01T06:30Z → the period is 00:00..06:00 (six closed hours)
INSERT INTO usage_source_streams (workspace_id, resource_kind, service_id, kind, source, expected_from, expected_through) VALUES
 ('tea-x', 'service', 'srv-zero',   'instance_seconds', 'instance', '2026-09-01T00:00Z', NULL),
 ('tea-x', 'service', 'srv-gap',    'instance_seconds', 'instance', '2026-09-01T00:00Z', NULL),
 ('tea-x', 'service', 'srv-ok',     'instance_seconds', 'instance', '2026-09-01T00:00Z', NULL),
 ('tea-x', 'service', 'srv-closed', 'instance_seconds', 'instance', '2026-09-01T00:00Z', '2026-09-01T03:00Z'),
 ('tea-x', 'service', 'srv-late',   'instance_seconds', 'instance', '2026-09-01T00:00Z', NULL),
 ('tea-other', 'service', 'srv-foreign', 'instance_seconds', 'instance', '2026-09-01T00:00Z', NULL);
INSERT INTO usage_source_health (workspace_id, resource_kind, service_id, kind, source, window_start, state)
SELECT 'tea-x', 'service', svc, 'instance_seconds', 'instance', '2026-09-01T00:00Z'::timestamptz + h * interval '1 hour', 'healthy'
FROM (VALUES ('srv-gap', 1), ('srv-gap', 2), ('srv-gap', 3), ('srv-gap', 4), ('srv-gap', 5),
             ('srv-ok', 0), ('srv-ok', 1), ('srv-ok', 2), ('srv-ok', 3), ('srv-ok', 4), ('srv-ok', 5),
             ('srv-closed', 0), ('srv-closed', 1), ('srv-closed', 2),
             ('srv-late', 0)) AS v(svc, h);
INSERT INTO usage_source_health VALUES ('tea-x', 'service', 'srv-late', 'instance_seconds', 'instance', '2026-09-01T03:00Z', 'degraded');
INSERT INTO usage_source_health VALUES ('tea-other', 'service', 'srv-foreign', 'instance_seconds', 'instance', '2026-09-01T00:00Z', 'degraded');
INSERT INTO usage_hourly (workspace_id, resource_kind, service_id, kind, window_start, quantity)
SELECT 'tea-x', 'service', 'srv-ok', 'instance_seconds', '2026-09-01T00:00Z'::timestamptz + h * interval '1 hour', 3600 FROM generate_series(0, 5) h;
