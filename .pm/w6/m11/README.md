# w6 · m11 — Live-verification sweep: close the m4/m5/m10 residuals on the restored cluster

**Worker:** worker6 **Goal:** every "not live-verified" residual w6 shipped with (m4's OpenBao purge, m5's live-cluster browser rerun, m10's Team page click-through) is replaced by recorded live evidence — or by a found-and-fixed bug. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                            | est | depends_on       |
| ---- | ---------------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Stand up the local stack (mock-cluster + Kratos/OpenFGA/OpenBao/CP-Postgres, `BEX_KRATOS_ADMIN_URL` wired)        | 40m | —                |
| t002 | m5 residual: real-browser rerun of the full workspace lifecycle (create → rename → `sudo delete …`) live          | 30m | t001             |
| t003 | m4 residual: live-verify the OpenBao tenant-secrets purge fires on workspace delete                               | 30m | t001             |
| t004 | m10 residual: Team page live click-through; extend `local-bex.mjs` with `workspaceMembers`                        | 45m | t001             |
| t005 | Simplify: `/simplify` over the code this milestone changed                                                        | 30m | t002, t003, t004 |
| t006 | Test coverage: meaningful tests for the behavior this milestone shipped (the stub's `workspaceMembers` path)      | 30m | t002, t003, t004 |
| t007 | Closeout: verify the DoD holds, mark done, move to `w6/done/m11/`                                                 | 15m | t006             |

## Definition of done

All three residual annotations (`w6/done/m4/README.md`, `done/m5/README.md`, `done/m10/README.md`) have closing evidence recorded in this milestone's README: the workspace lifecycle (create → rename → `sudo delete workspace <name>`) is re-driven in a real browser against the live cluster; a seeded OpenBao secret under the deleted workspace's `tenants/*` path is confirmed absent after delete; the Team page renders email-primary member rows from a live Kratos (screenshot). `dashboard/scripts/local-bex.mjs` serves `workspaceMembers` so future offline verifications cover the Team page.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w6` 2026-07-12 — the residual/follow-up sections of `w6/done/m4/README.md` (OpenBao purge "implemented + unit-tested but not live-verified"), `done/m5/README.md` (live-cluster browser rerun "blocked by a diagnosed local-cluster outage" — since fixed), and `done/m10/README.md` (no live click-through; `local-bex.mjs` lacks `workspaceMembers`).
- **Goal linkage:** GOAL.md #5 (multi-tenant) and #7 (security review) — the workspace-delete purge is a tenant-data-destruction path; "unit-tested but never fired against real OpenBao" is exactly the risk class the security review exists to burn down.
- **Expected outcome:** every w6 residual-risk annotation is replaced by recorded live evidence (or a real bug found and fixed — m5's rerun caught the `sudo`-phrase drift, so this class of work has a track record).
- **Why now:** the sole blocker (the Calico/CNPG local-cluster outage) is fixed; residuals compound — w6/m12 builds more settings UI on this same foundation.
- **Render parity:** **omitted** — no REST/GraphQL/MCP/UI surface change; pure verification of already-shipped surfaces (the verification itself is the parity check).
