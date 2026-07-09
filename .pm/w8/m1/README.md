# w8 · m1 — Metering pipeline: hourly usage rollups into the control-plane store

**Worker:** worker8 **Goal:** bex records what ran: an hourly metering loop in bex-api rolls Prometheus data (cAdvisor runtime, Traefik egress) and build-Job durations into durable per-service `usage_hourly` rows (`instance_seconds` by tier · `egress_bytes` · `build_seconds`) in the control-plane Postgres, keyed by workspace — so usage survives Prometheus's retention window and exists from day one of real tenancy. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                        | est | depends_on       |
| ---- | ------------------------------------------------------------------------------------------------------------ | --- | ---------------- |
| t001 | Usage schema migration (`0003_usage`): `usage_hourly` with an idempotent-upsert key                           | 30m | —                |
| t002 | Prometheus rollup queries: instance-seconds per service/tier (cAdvisor) + egress bytes (Traefik)              | 45m | —                |
| t003 | Metering loop in bex-api: hourly tick, idempotent upserts, restart catch-up; gated on store + `BEX_PROM_URL`  | 45m | t001, t002       |
| t004 | Build minutes: record completed build-Job durations (w1/m5 BuildKit Jobs) as `build_seconds` rows             | 30m | t001             |
| t005 | Month-to-date aggregation in `internal/store` (per workspace / service / kind) + fake-clock unit tests        | 30m | t001             |
| t006 | Local acceptance: mock cluster + sample app, verify rows accrue across two windows                            | 30m | t003, t004, t005 |
| t007 | Simplify: `/simplify` over the code this milestone changed                                                    | 30m | t006             |
| t008 | Test coverage: meaningful tests for rollup correctness, idempotency, catch-up                                 | 45m | t006             |
| t009 | Closeout: verify DoD, mark done, move milestone to `done/`                                                    | 15m | t008             |

## Definition of done

On a cluster with `BEX_CP_DB_URI` and `BEX_PROM_URL` set, a running App accrues `usage_hourly` rows every hour — `instance_seconds` matching its tier and uptime, `egress_bytes` matching Traefik's numbers, and `build_seconds` after a build-from-git deploy — and re-running the rollup for the same window (including across a bex-api restart) changes nothing (idempotent upserts).

## Source + Goal linkage

- **Source:** `/pm-brainstorm w8` 2026-07-09 — `GOAL.md` #5's usage-metering half, owned by no workstream before w8. What to meter verified live 2026-07-09 against render.com/pricing, docs/build-pipeline, docs/outbound-bandwidth (per-second compute · bandwidth · pipeline minutes) and `.pm/w6/RESEARCH-workspaces.md` finding 6.
- **Goal linkage:** V0 roadmap item 5 ("Multi tenant and usage metering"); underwrites the vision's free-tier/"sleep = free" economics — you can't show sleep saves anything without measured runtime.
- **Expected outcome:** durable per-workspace usage history in the control-plane Postgres, accruing hourly for every service, ready for m2's API and m3's dashboard page.
- **Why now:** the substrate just finished shipping (w1/m2 store, w1/m5 in-cluster builds, w1/m8 tier catalog, w3 Prometheus, w6/m1 plans), and w1/m9 mints real tenants next — metering that starts after tenants arrive means unmetered history that can never be reconstructed.
- **Render parity: omitted** — pure pipeline, no REST/GraphQL/MCP/UI surface change (the surfaces come in m2/m3, which include the parity task). Parity of _what_ is metered is by design: instance-seconds, egress bytes, and build seconds are exactly Render's three meters.
- **Known risk:** prod activation requires `BEX_CP_DB_URI` flipped on — the still-open w1/m2/t007; m1 works locally regardless and adds a second consumer's worth of pressure to close t007. Until w1/m9 lands, rows attribute to the default workspace — the schema keys by workspace from day one so attribution starts working the moment m9 does.
