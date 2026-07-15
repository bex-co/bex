# w2 · m38 — Full deploy status lifecycle + transition timestamps

**Worker:** worker2 **Goal:** bex deploy rows expose the evidence-backed lifecycle states and transition timestamps Render-trained clients expect, so REST/GraphQL/MCP and the dashboard can explain where a deploy failed. **Status:** done (2026-07-15)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Store schema: updated-at + transition facts — **DONE** | 40m | — |
| t002 | Define the evidence-backed deploy transition model — **DONE** | 40m | t001 |
| t003 | Persist build-phase progress and failure — **DONE** | 50m | t002 |
| t004 | Persist pre-deploy · rollout · deactivation transitions — **DONE** | 60m | t002 |
| t005 | REST · GraphQL · MCP status and filter parity — **DONE** | 45m | t003, t004 |
| t006 | Render parity — **DONE** | 30m | t005 |
| t007 | Simplify — **DONE** | 30m | t006 |
| t008 | Transition and dashboard test coverage — **DONE** | 45m | t006 |
| t009 | Closeout — **DONE** | 15m | t008 |

## Outcome (2026-07-15)

Deploy rows now persist Render's full eleven-state vocabulary from current-generation App/Job evidence, with an explicit no-regression transition table and truthful skips for phases the polling control plane never observed. `updated_at` is backfilled for old rows and advances only on real fact changes; first execution and terminal instants remain distinct. App-generation ordering, a per-App database lock, and a partial unique index make overlapping deploys newest-wins even when requests reach Postgres out of order. A new live deploy atomically deactivates the prior live revision while preserving its original finish time.

REST, GraphQL, and MCP expose the same status/timestamps and status plus created/updated/finished time filters. The dashboard labels and filters all eleven states, timestamps the current fact from `updatedAt`, preserves the earlier live step on deactivation, polls every open state, and stops on every terminal state. Render's commit-author timestamp remains explicit follow-up `.pm/w2/011.md`; no adapter invents it.

Verification: disposable PostgreSQL migration/store/deploy suites; `go test ./...`; `go build ./...`; `make lint-backend` (zero issues); dashboard lint/typecheck; 187 dashboard test files / 1,133 tests; production dashboard build.

## Definition of done

Deploy rows move through the truthful subset of Render's eleven-state vocabulary as build, pre-deploy, rollout, cancel, and deactivation work occurs, with `updatedAt` advancing on each real transition. Filters accept every stored status and the dashboard timeline renders those facts. Surface-parity and failure-mode tests pass, and no adapter invents data absent from the store.

## Source + Goal linkage

- **Source:** `w5/m29` Render-parity audit, 2026-07-14; replaces the lifecycle portion of the referenced but never materialized `w2/m32` scope. w9/001 independently shipped commit id/message provenance before this was filed.
- **Goal linkage:** Render API/dashboard parity and trustworthy deploy debugging for human and agent clients.
- **Expected outcome:** a failed deploy identifies its failing phase without inference from logs alone; existing commit id/message provenance stays intact.
- **Why now:** `w5/m29` shipped the consuming detail-page shape and recorded the missing lifecycle facts explicitly; delaying the store contract would have made more consumers depend on the then-current coarse four-state projection. Render parity is included because REST, GraphQL, MCP, and dashboard surfaces all change.
