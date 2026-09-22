# w4 · m134 — Reach workspace-scoped credentials and event details

**Worker:** worker4 **Goal:** Workspace selection reaches every supported operation **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Registry credential workspace selectors — **DONE** | 45m | — |
| t002 | Event hydration workspace selectors — **DONE** | 30m | t001 |
| t003 | Dashboard credential workspace propagation — **DONE** | 35m | t002 |
| t004 | Render parity — **DONE** | 20m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 25m | t005 |
| t007 | Closeout — **DONE** | 5m | t006 |

## Definition of done

Registry credential get/update/delete and event get work in an explicitly selected member workspace through REST, GraphQL, and MCP, without weakening authorization or changing omitted-workspace defaults. Dashboard credential operations send their selected workspace. Backend and dashboard checks pass.

## Source + Goal linkage

- **Source:** [w4/128 investigation](source-128.md), promoted 2026-09-22 after identifying dashboard callers and the separate event hydration path.
- **Goal linkage:** ADR008 hosting control-plane reliability and ADR012 workspace authorization.
- **Expected outcome:** Credentials created in a second workspace remain manageable, and webhook event IDs can be hydrated in their owning workspace.
- **Why now:** Existing default-only adapters strand durable credentials and break event detail lookup.
- **Scope:** Optional adapter selectors and dashboard propagation; no global by-ID store lookup or authorization redesign. MCP already binds workspaceId; verify it rather than duplicate the selector. Render parity applies to the public API and dashboard changes.

## Verification notes

REST event hydration already honored ownerId and passed its strict-query extension gate; only GraphQL needed the binding. MCP retains its standard workspaceId middleware. CLI telemetry deliberately stores client-reported workspace separately from authenticated attribution (`clitelemetry/service.go:24–27`); no analogous unreachable resource exists.

Dashboard list and create also omitted ownerId; all five hooks now bind the selected workspace, and regression coverage switches from B to C to exercise callback dependencies. The offline schema dump/codegen and both TypeScript checks passed, as did focused ESLint and 30 registry tests. No visual layout changed.

Backend full suite passed with local Postgres/OpenFGA/OpenBao (`/tmp/bex-w4-m134-backend.log`), backend lint has zero issues (`/tmp/bex-w4-m134-lint.log`), and the strict-query acceptance regression passed (`/tmp/bex-w4-m134-validator-final.log`). A temporary overlay reverting the adapters fails the new registry/event tests (`/tmp/bex-w4-128-mutation.log`). The validator test initially used an incomplete Render PATCH body; corrected fixture fields satisfy the pinned contract, without changing production body semantics.

The complete dashboard baseline passed 437 files / 3,598 tests (`/tmp/bex-w4-128-dashboard-full.log`). Reuse and quality reviews found no changes needed. Efficiency review identified an initial unscoped credential query before workspace selection; follow-up applies the existing skip/loading pattern with focused unresolved-to-selected coverage.

All three simplify reviews completed. The workspace-wait cleanup passed all 31 focused tests across 8 files, including unresolved→selected and B→C selection changes; the full dashboard baseline above preceded this bounded cleanup.

Final dashboard TypeScript (application and test projects) and targeted ESLint passed after cleanup. Adapter tests use in-memory stores and secrets; the complete backend suite separately runs native Postgres/OpenFGA and local OpenBao integration coverage. No paid or production credential fixture was created. No deployment verification is claimed.

Concurrent landing: `5da881aff` shipped the server adapter bindings and closed 128 during this milestone. The merge retains that equivalent helper-based implementation and its tests; m134 supplies the missing strict REST ownerId allowance, additional authorization/transport coverage, and dashboard completion. Concurrent note 140 is closed by this same dashboard work.

Post-merge API (real local dependencies), events, and registry credential package suites passed (`/tmp/bex-w4-m134-postmerge.log`); backend lint again reports zero issues.
