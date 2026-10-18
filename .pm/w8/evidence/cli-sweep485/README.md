# PG paid-feature refusals + request-log dimensions — sweep485

Passing functional coverage, no new issue. Same isolated session as sweeps 454–484, `bex-canary` + `tian-personal`, HEAD checkout build, 2026-10-18. Inline loop; held ship state unchanged (see sweep484).

Postgres paid-feature gates refuse cleanly at create with coded, plan-explaining errors and no resource created (personal workspace count stayed 0): `--high-availability` → `POSTGRES_HA_PLAN_UNSUPPORTED` ("plan free has 100m CPU"), `--read-replica` → `POSTGRES_READ_REPLICA_PLAN_UNSUPPORTED` (≥0.5 CPU). Request-log dimensions: raw request lines carry exactly `type/resource/method/statusCode` labels, so `--type request --instance <real-instance>` is an honest empty (the instance dimension exists on app lines and in discovery, not on request lines); a bogus instance is likewise empty rather than an error.

Cleanup: exact-owned `srv-dbaijb1n2qrc73fmlstg` deleted by ID (404-verified); baseline deltas are concurrent foreign churn. No plan mutations (both PG probes were refusals at create). Session remains active; next inline sweep is 486; `f62d6740e` + evidence 477–485 await a green main.
