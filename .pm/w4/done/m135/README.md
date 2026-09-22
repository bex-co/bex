# w4 · m135 — Keep usage attribution after resource deletion

**Worker:** worker4 **Goal:** Charges retain names and honest lifecycle markers **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Capture names during metering and resolve deletion independently — **DONE** | 55m | — |
| t002 | Resolve sandbox metadata from durable lifecycle evidence — **DONE** | 40m | t001 |
| t003 | Render parity — **DONE** | 15m | t002 |
| t004 | Simplify — **DONE** | 15m | t003 |
| t005 | Test coverage — **DONE** | 30m | t004 |
| t006 | Closeout — **DONE** | 5m | t005 |

## Definition of done

A datastore metered and deleted before any usage-page read retains its name. Known-absent services/datastores are marked deleted even if an old name was never captured; failed inventory reads do not imply deletion. Sandbox metadata separates historical labels from meter lifecycle evidence and provides a tier fallback where available. REST/GraphQL/MCP share the result; no quantities, rates, or invoice behavior change. Backend tests including real SQL and lint pass.

## Source + Goal linkage

- **Source:** [129 QA investigation](source-129.md), promoted 2026-09-22. Related w2/blocked/m96 introduced retention but read-time capture misses resources deleted before a usage read; this milestone fixes that implementation gap without changing w2's board.
- **Goal linkage:** ADR008 reliable hosting control plane; ADR023 usage attribution.
- **Expected outcome:** Users can distinguish historical charges from current resources without needing to visit billing before deleting a resource.
- **Why now:** QA identified fresh deleted datastore rows without names, which disproves the assumption that presentation reads enumerate every resource during its lifetime.
- **Scope:** Attribution/lifecycle metadata only. Lost historical names remain unknown; no guessed names from IDs, broad audit backfill, billing rate changes, or production resource mutation. Render parity applies to the shared API and existing Charges rendering.

## Verification notes

Store metadata tests passed against a fresh local Postgres database (`/tmp/bex-w4-129-metadata.log`), covering running/suspended/terminated, a missing workload after a successful complete list, historical labels with unknown lifecycle, and cross-workspace same-ID isolation. No migration or production resource mutation is needed. Existing Charges rendering already handles a bare ID with a deleted marker and a descriptive sandbox label; the focused 40-test suite passes with regressions for both.

Usage package regressions passed (`/tmp/bex-w4-129-usage.log`). A Go overlay bypassing collector name capture failed both the deleted-before-first-read assertion and the capture-failure no-write assertion (`/tmp/bex-w4-129-mutation.log`). The new tests also preserve all seeded usage quantities across resolution and verify all three API adapters.

Render comparison: the public dashboard guide documents accrued monthly charges, but not retained-name/deleted-row semantics. ADR023 therefore labels this as bex attribution behavior without claiming exact Render UI parity. REST/GraphQL/MCP share the tested resolver, and the dashboard already consumes both fields. Unknown historical names/lifecycle remain explicitly limited.

All three simplify reviews completed: removed legacy fake label state and redundant metadata maps/assertions, skipped irrelevant inventories, and batched failure-health writes per workspace. Full backend integration passed with local Postgres/OpenFGA/OpenBao (`/tmp/bex-w4-m135-backend.log`); final usage suite passed after the bounded efficiency cleanup, and final backend lint reports zero issues (`/tmp/bex-w4-m135-lint-final.log`). Dashboard runtime/schema is unchanged; 40 focused Charges tests pass. Collector lifecycle tests use fake Kubernetes/Prometheus plus an in-memory usage store; sandbox metadata SQL runs against real Postgres. No live production verification is claimed.

Concurrent landing: `3757d0ff1` independently corrected deletion markers while this milestone ran. The merge preserves its tests and lifecycle semantics; this milestone additionally captures names in the collector and consolidates sandbox name/phase/tier into one query. The earlier capture gap is fixed without adding lifecycle hooks across four packages.

Merge verification: both sets of usage regressions pass (`/tmp/bex-w4-129-usage-merged.log`), and the merged single-query metadata tests pass against local Postgres (`/tmp/bex-w4-129-metadata-merge.log`). The first combined full run exposed an unrelated agent-session metric-test clock-resolution race: an immediate fake failure can measure zero duration, which the observer deliberately drops. The test now models a one-second failure with virtual time and asserts the recorded sample; 100 normal and 100 race-enabled runs pass. The combined full backend suite passes with local Postgres/OpenFGA and native OpenBao (`/tmp/bex-w4-m135-backend-final.log`); an unresponsive owned Docker OpenBao was replaced with an isolated native test instance. Final lint reports zero issues (`/tmp/bex-w4-m135-lint-final2.log`). Production metric behavior is unchanged.
