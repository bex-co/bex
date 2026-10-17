# w4 · m221 — Preserve creation placement when lookup reads fail

**Worker:** worker4 **Goal:** a failed Project/Environment lookup preserves the customer's selected placement and offers read recovery before creation continues. **Status:** todo

**Estimate:** 2h35 (6 tasks). **Severity:** major. Filing only; no product fix included.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Preserve selection and expose placement-read recovery | 45m | — |
| t002 | Audit the three selector clients and their preserved controls | 25m | t001 |
| t003 | Render parity | 15m | t002 |
| t004 | Simplify | 10m | t003 |
| t005 | Test failed reads, retained intent and creation requests | 40m | t003 |
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
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md)'s usable hosting dashboard, [ADR032](../../../docs/ADR032-environments.md)'s placement model and [ADR018](../../../docs/ADR018-render-parity.md)'s existing creation parity. A contextual create must preserve the customer's destination through a read failure.
- **Expected outcome:** lookup errors preserve placement intent, explain unavailable metadata and let the customer retry reads without an unintended unassigned create.
- **Why now:** five controlled read-failure captures clear valid selections, and one real native Free create plus an independent read confirms the resulting unassigned placement. The backend lookups themselves succeeded; these were browser-local failure substitutions, not a production outage.
- **Scope:** one shared selector, two lookup branches, three production clients (five App types, Postgres and Key Value). Keep global hooks/Select, API contracts and intentional No environment semantics unchanged.
- **Dedupe:**88 actual open/blocked milestone titles plus2 evidence README candidates, whole-board text, DO_NOT_DO, recent40 product history and targeted selector history reviewed. m218 owns plan-catalog recovery and m219 pre-handler readiness. w4/173's accurate No environment hint remains a passing contract; w5/m41's contextual creation works when reads succeed. See the disposition in the finding.
- **Render parity:** included for the UI/request-intent change. [Render's Projects docs](https://render.com/docs/projects) describe choosing Project/Environment during creation. Authenticated Render outage behavior was not measured; retain bex's existing API placement and deletion semantics.
