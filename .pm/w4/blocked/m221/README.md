# w4 · m221 — Preserve creation placement when lookup reads fail

**Worker:** worker4 **Goal:** a failed Project/Environment lookup preserves the customer's selected placement and offers read recovery before creation continues. **Status:** blocked — t001–t005 done 2026-10-17; t006 (owned live replay and Free create on a deployed build) open

**Estimate:** 2h35 (6 tasks). **Severity:** major. Implemented 2026-10-17; awaiting the deployed-build replay (t006).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Preserve selection and expose placement-read recovery — **DONE** | 45m | — |
| t002 | Audit the three selector clients and their preserved controls — **DONE** | 25m | t001 |
| t003 | Render parity — **DONE** | 15m | t002 |
| t004 | Simplify — **DONE** | 10m | t003 |
| t005 | Test failed reads, retained intent and creation requests — **DONE** | 40m | t003 |
| t006 | Replay the owned journey and close out | 20m | t004, t005 |

## Definition of done

- Recreate the owned empty Project/Environment and follow its contextual New Service link. Preserve the observed healthy controls: both selections agree with the URL across all five App type previews at1440/390; a fresh static-form load resolves the same context.
- Repeat [a856's exact per-operation browser-local read failures](evidence.json) on fresh desktop/mobile web forms. An Environment-list failure retains the selected Environment ID; a Project-list failure retains both IDs. Present an explicit localized lookup error and read-only Retry. Resolve selected labels from real data; pending/unavailable metadata must not masquerade as a user-selected No project/No environment.
- Remove the interception and use Retry. The same selected labels return without reselecting them, and the customer's other form fields remain. Retry re-reads metadata and does not create a resource. Only a settled, error-free list can invalidate a selection as missing; an explicit user clear retains its existing meaning.
- Continue the recorded Free image-web create after recovery. Its native request includes the desired `environmentId`, and the returned service plus an independent GraphQL read have the intended Project/Environment IDs. The recorded faulty request omitted that field and created an unassigned service. Preserve the existing backend semantics for intentional No environment.
- Preserve the single-service Manage resources recovery control, which assigns the owned service to the intended Project/Environment while retaining its original Live deploy. PG/KV failures, warm-cache and partial/transport errors, stale-ID and pending-choice variations are t002/t005 verification work, not additional observed failures.
- Delete the owned service and Project/Environment; exact service/deploy/project/environment reads and the public URL return404, all seven starting resource-ID sets match, and the App/serving/build children are gone across namespaces. Revoke only the sweep's session. Close this milestone only after the fixed-build evidence and required checks exist.

## Source + Goal linkage

- **Source:** functional `$qa-find-bugs w4`, a856, 2026-10-17 UTC; [finding](finding.md), [exact scripts and complete safe receipts](evidence.json). Research main `24e10c7f8`; the selector still has the failure-blind invalidation effect there.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md)'s usable hosting dashboard, [ADR032](../../../../docs/ADR032-environments.md)'s placement model and [ADR018](../../../../docs/ADR018-render-parity.md)'s existing creation parity. A contextual create must preserve the customer's destination through a read failure.
- **Expected outcome:** lookup errors preserve placement intent, explain unavailable metadata and let the customer retry reads without an unintended unassigned create.
- **Why now:** five controlled read-failure captures clear valid selections, and one real native Free create plus an independent read confirms the resulting unassigned placement. The backend lookups themselves succeeded; these were browser-local failure substitutions, not a production outage.
- **Scope:** one shared selector, two lookup branches, three production clients (five App types, Postgres and Key Value). Keep global hooks/Select, API contracts and intentional No environment semantics unchanged.
- **Dedupe:**88 actual open/blocked milestone titles plus2 evidence README candidates, whole-board text, DO_NOT_DO, recent40 product history and targeted selector history reviewed. m218 owns plan-catalog recovery and m219 pre-handler readiness. w4/173's accurate No environment hint remains a passing contract; w5/m41's contextual creation works when reads succeed. See the disposition in the finding.
- **Render parity:** included for the UI/request-intent change. [Render's Projects docs](https://render.com/docs/projects) describe choosing Project/Environment during creation. Authenticated Render outage behavior was not measured; retain bex's existing API placement and deletion semantics.

## Implementation record (2026-10-17)

- **t001:** `ProjectEnvironmentSelector` treats a list as authoritative only when it settled without an error (`!loading && !error`); only then may a missing Project or Environment be cleared. Failed, partial (data plus errors), transport and pending reads keep both IDs. A kept selection that its list has not named shows "Loading…" while pending and "Selected (name unavailable)" on error, never No project/No environment. A localized `ReadUnavailable` alert ("Couldn't load projects or environments…") offers Retry through `useCatalogRetry`, which re-reads only the failed list(s). Explicit clears, Project-change clearing and the stale-ID clear on an error-free list are unchanged; URL defaults are not reapplied. `ReadUnavailable` is the new generic form-region alert that m218's `PlanCatalogUnavailable` now wraps. Global hooks, Select and API contracts are unchanged.
- **t002 census:** one selector, two Select sites, three mounts: `routes/services.new.tsx` (web/static/private/worker/cron), `routes/keyvalue.new.tsx` and `create-database-dialog.tsx` (Postgres). Each serializes its nullable `environmentId` as optional (`create-service-input.ts:196`, `keyvalue.new.tsx:127`, `create-database-dialog.tsx:170`), so a kept ID reaches the request. Aliases `/r/new`, `/new/redis` → `/keyvalue/new`; `/d/new`, `/new/database` → the Postgres dialog; service-type Render aliases → `/services/new`. PG/KV start with local null placement (no URL preselection); their failure path is the same component. Blueprint and Environment Group pickers do not mount it. Source-only: PG/KV were not live-driven.
- **t003:** ADR018's create-service row records the placement-read behavior; Render's failed-read behavior was not compared.
- **t004:** three /simplify passes (shared alert extracted, listed checks computed once, placeholder only while unsettled, neutral copy, global test setup and workspace mock reused).
- **t005:** `project-environment-selector-read-failure.test.tsx` runs the real selector and list hooks on a real Apollo client with the app's cache config and defaults; only the network is faked. Covered: Environments and Projects failures (GraphQL and transport) keep the IDs and an unrelated draft, and Retry restores the labels with one extra read of the failed list only; an empty list with errors; the pending label; a warm-cache refresh failure; a deliberate No environment; a Project change during a failure; error-free stale-ID clears; and the real create chain (`buildCreateServiceInput` → `useCreateService`) whose `CreateService` wire variables carry `environmentId: evm-1` after Retry. Nine fail against the old error-blind selector; the controls pass on both. `services.new.test.tsx` adds a route-level request check. Dashboard suite (5148 tests), lint and typecheck pass.
