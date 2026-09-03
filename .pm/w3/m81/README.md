# w3 · m81 — Deleting-service lifecycle contract: consistent visibility + bounded teardown

**Worker:** worker3 **Goal:** deleting a service produces one coherent tenant contract across list, by-id REST/GraphQL/MCP/dashboard surfaces and the serving URL, while teardown that exceeds its expected window becomes bounded and observable instead of leaving a hidden resource indefinitely. **Status:** todo

## Tasks (in order)

| id   | title                                                               | est | depends_on      |
| ---- | ------------------------------------------------------------------- | --- | --------------- |
| t001 | Freeze the deleting-service read + finalizer contract               | 45m | —               |
| t002 | Apply deletion visibility and dead-URL semantics across all surfaces | 60m | t001            |
| t003 | Bound and surface static-site finalizer overruns                    | 60m | t001            |
| t004 | Live deletion timing + cross-surface E2E                            | 45m | t002, t003      |
| t005 | Render parity sweep for deleting resources                          | 30m | t004            |
| t006 | Simplify the changed lifecycle code                                 | 20m | t005            |
| t007 | Test coverage for visibility, timeout, and failure modes            | 45m | t005            |
| t008 | Closeout                                                            | 10m | t006, t007      |

## Definition of done

After deleting a static site, its list row, REST service/routes/deploys/headers reads, GraphQL service read, MCP reads, dashboard detail route, and advertised URL all follow the same documented contract within a stated window. A finalizer that exceeds that window has a bounded outcome that is either tenant-visible or makes the resource consistently absent from every tenant surface; it cannot leave an unbounded `Deleting` record hidden from lists while by-id reads advertise a withdrawn URL. Tests cover normal finalization and teardown failure, and a disposable live fixture proves the timing and cross-surface result.

## Source + Goal linkage

- **Source:** promoted from `w3/m46/t009` during the 2026-08-31 w3 cleanup. A production QA static site disappeared from lists and lost its route/certificate immediately after delete, but service/routes/deploys/headers REST reads, GraphQL, and the dashboard continued serving `phase: Deleting` plus a dead URL for 2+ hours. The original fixture eventually finalized, but the source contract still permits the inconsistency without a tenant-visible bound.
- **Goal linkage:** Render-compatible service lifecycle semantics and hosted-platform trust. A tenant should see one answer to “does this service still exist?”, regardless of which bex surface they use.
- **Expected outcome:** delete visibility is consistent across REST/GraphQL/MCP/UI and serving metadata; stuck static-site cleanup cannot strand a hidden quota-consuming resource with a live by-id detail forever.
- **Why now:** m46's original implementation and its Render-parity/simplify/test closing tasks are already complete. The follow-up spans API projection, dashboard behavior, operator finalization, docs, failure tests, and live timing, so keeping it as one 45-minute task after those closing tasks understated the work and made m46's dependency chain false.
- **Render parity task included:** deleting-service behavior is tenant-facing across REST, GraphQL, MCP, and dashboard surfaces, so t005 verifies they move together and compares the result with Render rather than silently choosing a bex-only contract.

## Evidence carried forward

- The affected production fixture was `srv-da7tf87krsvc73c3mcng`: serving route/certificate gone, list row gone, but every by-id surface returned 200 and the dashboard rendered `Service Unknown` with the dead URL for 2+ hours.
- Controls in the same workspace finalized normally: Key Value in ~1 second, private service in under 6 seconds, cron job in 15 seconds. This isolates the observed hang to the static-site cleanup path rather than App deletion generally.
- The original fixture now returns 404, so this milestone does not claim an active incident. It closes the still-present unbounded contract and requires a fresh disposable timing probe.

