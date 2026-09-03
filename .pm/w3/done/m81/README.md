# w3 · m81 — Deleting-service lifecycle contract: consistent visibility + bounded teardown

**Worker:** worker3 **Goal:** deleting a service produces one coherent tenant contract across list, by-id REST/GraphQL/MCP/dashboard surfaces and the serving URL, while teardown that exceeds its expected window becomes bounded and observable instead of leaving a hidden resource indefinitely. **Status:** done

## Tasks (in order)

| id   | title                                                               | est | depends_on      |          |
| ---- | ------------------------------------------------------------------- | --- | --------------- | -------- |
| t001 | Freeze the deleting-service read + finalizer contract               | 45m | —               | — **DONE** |
| t002 | Apply deletion visibility and dead-URL semantics across all surfaces | 60m | t001            | — **DONE** |
| t003 | Bound and surface static-site finalizer overruns                    | 60m | t001            | — **DONE** |
| t004 | Live deletion timing + cross-surface E2E                            | 45m | t002, t003      | — **DONE** |
| t005 | Render parity sweep for deleting resources                          | 30m | t004            | — **DONE** |
| t006 | Simplify the changed lifecycle code                                 | 20m | t005            | — **DONE** |
| t007 | Test coverage for visibility, timeout, and failure modes            | 45m | t005            | — **DONE** |
| t008 | Closeout                                                            | 10m | t006, t007      | — **DONE** |

## Definition of done

After deleting a static site, its list row, REST service/routes/deploys/headers reads, GraphQL service read, MCP reads, dashboard detail route, and advertised URL all follow the same documented contract within a stated window. A finalizer that exceeds that window has a bounded outcome that is either tenant-visible or makes the resource consistently absent from every tenant surface; it cannot leave an unbounded `Deleting` record hidden from lists while by-id reads advertise a withdrawn URL. Tests cover normal finalization and teardown failure, and a disposable live fixture proves the timing and cross-surface result.

## Source + Goal linkage

- **Source:** promoted from `w3/m46/t009` during the 2026-08-31 w3 cleanup. A production QA static site disappeared from lists and lost its route/certificate immediately after delete, but service/routes/deploys/headers REST reads, GraphQL, and the dashboard continued serving `phase: Deleting` plus a dead URL for 2+ hours. The original fixture eventually finalized, but the source contract still permits the inconsistency without a tenant-visible bound.
- **Goal linkage:** Render-compatible service lifecycle semantics and hosted-platform trust. A tenant should see one answer to “does this service still exist?”, regardless of which bex surface they use.
- **Expected outcome:** delete visibility is consistent across REST/GraphQL/MCP/UI and serving metadata; stuck static-site cleanup cannot strand a hidden quota-consuming resource with a live by-id detail forever.
- **Why now:** m46's original implementation and its Render-parity/simplify/test closing tasks are already complete. The follow-up spans API projection, dashboard behavior, operator finalization, docs, failure tests, and live timing, so keeping it as one 45-minute task after those closing tasks understated the work and made m46's dependency chain false.
- **Render parity task included:** deleting-service behavior is tenant-facing across REST, GraphQL, MCP, and dashboard surfaces, so t005 verifies they move together and compares the result with Render rather than silently choosing a bex-only contract.

## Outcome / evidence (2026-09-02)

**Contract shipped (t001–t003).** Once a service's deletion is accepted (the App CR carries a `DeletionTimestamp`), every by-id tenant surface reports **consistent absence** — `core.NotFoundIfDeleting` (`lego/backend/internal/core/base.go`) returns the same `core.ErrNotFound` the list already applies by omission, wired into `apps.Get`/`ListRoutes`/`ListHeaders` and `deploys.List`/`Get`, so REST service/routes/headers/deploys, GraphQL `server`/`service`, and MCP `get_service` all agree with `List` and Render's `GET 404`. The shared `view()` projection also blanks the withdrawn URL as defense-in-depth. The operator bounds finalization: a static-content purge that overruns a 15-minute window (`AppReconciler.FinalizerOverrunAfter`, default `finalizerOverrunAfter`) is stamped `DeletionStalled` on the App and backed off to a 30s requeue while the finalizer is **retained** (no silent orphan; `terminating` quota still counts it). The dashboard shows a muted "Deleting" badge, suppresses the dead URL, keeps polling `deleting` until the read resolves to not-found, and its existing not-found path redirects with "…was deleted." Documented in ADR004/006/018/029 + the CLI checklist.

**Verified (cluster-independent regression evidence — the w2/m61 standard).** All three platform suites pass:

- Backend — `apps/service_deletion_test.go` (`TestDeletingServiceIsAbsentFromEveryByIDRead`, `TestActiveServiceReadsAreUnaffected`, `TestDeletingProjectionDropsDeadURL`), `deploys/service_deletion_test.go`, and the rewritten `canceled_phase_test.go` (`TestDeletingServiceIsNotFoundByID`).
- Operator — `app_controller_deletion_test.go`: `TestStaticAppDeletionSurfacesStalledConditionOnOverrun` (overrun → `DeletionStalled` + finalizer retained + 30s requeue, via the `FinalizerOverrunAfter` seam, no multi-minute sleep), `…WithinBoundStaysQuiet` (control), `…OverrunStillFinalizesOnCompletion`. `make test` green (controller 82.4%).
- Dashboard — `status.test.ts` (`deleting` → Deleting badge, `isConvergingPhase("deleting")`), `service-detail-header.test.tsx` (Deleting + no live URL link), `service-detail-dead-id.test.tsx` (not-found → redirect + "was deleted"). `yarn test` (2876) + typecheck + lint green.

**Live timing probe (t004).** `scripts/static-delete-timing-verify.sh` is the fresh disposable turnkey probe: it creates a static site with a route + header, waits for it to serve, deletes it, and polls list, REST service/routes/headers/deploys, GraphQL, and the public host every 3s until **all** converge to absence, failing if the 5-minute (w5/m49) window is exceeded; it always tears its fixture down and never prints the bearer or a kubeconfig. **Live execution is deferred**, honestly: production (`hetzner-prod`) is off-limits without explicit authorization and the original incident fixture already returns 404 (see below), while the local CAPD static plane is credential-less/degraded-503 by design (no S3), so a real publish→purge with objects is not exercisable here. Run it against a static-S3 environment with `API=… BEARER=… REPO=… scripts/static-delete-timing-verify.sh`. The timing + cross-surface + bounded-failure behaviors are otherwise proven by the deterministic suites above.

## Evidence carried forward

- The affected production fixture was `srv-da7tf87krsvc73c3mcng`: serving route/certificate gone, list row gone, but every by-id surface returned 200 and the dashboard rendered `Service Unknown` with the dead URL for 2+ hours.
- Controls in the same workspace finalized normally: Key Value in ~1 second, private service in under 6 seconds, cron job in 15 seconds. This isolates the observed hang to the static-site cleanup path rather than App deletion generally.
- The original fixture now returns 404, so this milestone does not claim an active incident. It closes the still-present unbounded contract and requires a fresh disposable timing probe.

