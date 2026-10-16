# PostgreSQL parameter replacement, native rename and clearing — sweep325

[Complete native/API/SQL journey and cleanup](native-api-sql-journey.json) · [finite guarded driver](journey-driver.py.txt) · [source results](source-results.json).

Published Bex0.3.2/a6350da0 creates one fresh explicit Free PostgreSQL18 in bex-canary, Frankfurt, pool none, no replicas/HA. Native create exits0 and proves returned ID/name/workspace/plan ownership. Postgres inventory was empty and the original six services were intact before create. Source is 28948cadeb74d74f050274b9247c9d144227244f; serving revision is unobserved. The CLI oracle remains unchanged Render2.27.0/a764810a7682.

A real local psql18.6 session through native Bex seeds one owned marker row. Each subsequent fresh connection uses -X and ON_ERROR_STOP=1 and reads the marker, row count, server version, TimeZone, work_mem and pg_postmaster_start_time. Inherited PG overrides are removed. No connection-info response, URI, password or personal startup file is captured. Bex supplies its per-command temporary CA; this driver saves no certificate or connection credential. The local personal root certificate was absent at preflight.

## Observed results

All six final SQL/declared-state comparisons pass on server_version_num180006:

| Step | Fresh native SQL result | REST declared map |
| --- | --- | --- |
| Baseline | Etc/UTC, work_mem4MB /4096kB, marker row intact | Empty |
| API-only two-setting PATCH | Europe/Paris, work_mem8MB /8192kB | timezone=Europe/Paris, work_mem=8MB |
| Bex native rename only | Same two effective settings and row | Same map, version18 |
| Unmodified Render native rename only | Same two effective settings and row | Same map, version18 |
| API-only replacement with one setting | Asia/Tokyo, original work_mem4MB | timezone=Asia/Tokyo only |
| API-only clear with empty object | Original Etc/UTC and work_mem4MB | Empty |

The native rename commands send only --name; neither parameterOverrides nor version is explicitly supplied. Both exit0, and SQL is selected by each freshly unique owned name. The three parameter changes use PATCH /v1/postgres/{owned-id} with a parameterOverrides object; these are explicitly API-only coverage because the pinned native rename command has no parameter flag. GET detail and /parameters agree with the requested declaration after every final comparison. The latter endpoint is the declared map, rather than the full pg_settings observation.

The first read immediately after replacing two settings with one still observed Europe/Paris/8MB while metadata already declared Asia/Tokyo. A later fresh SQL read converged to Asia/Tokyo/4MB. Both receipts are retained; the early read is not filed as a defect. The harmless marker row remains exact throughout. These queries do not execute memory-heavy work. No operator-owned, role, authentication or logging setting is changed.

The focused existing ParameterSpec declaration test and withdrawn-field projection test both pass. The latter is the implementation boundary fixed in [w7/032](../../../w7/done/032.md); this sweep adds hosted SQL evidence rather than filing a duplicate. The declared-vs-observed distinction is also covered by [w6/m133](../../../w6/done/m133/README.md). No new defect or implementation is claimed.

## Scope and cleanup

This covers native create, native psql by ID/name, both native rename clients and API-only set/replace/clear with real SQL readback. It does not cover raw Render SQL trust, pooling/HA/replicas, paid plans, load, restart-required parameters, browser writes, direct GraphQL/MCP writes or a deployed SHA.

Before deletion the exact owned detail confirms no replicas and pool none; the ledger has no dependent fixture. Native Bex delete exits0. Detail404, native Postgres list empty, original six service IDs intact and database quota0 used/0 terminating establish cleanup. Copied Render configuration is removed after every native Render command. The QA session remains isolated for the ongoing loop. This evidence is uncommitted pending another ship instruction.
