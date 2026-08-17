# w2 · m72 — Build supersede semantics: run-to-completion + latest-pending slot (ADR060 D1)

**Worker:** worker2 **Goal:** frequent pushes can no longer starve deployment — a running build always runs to completion, a newer push coalesces into a single per-App pending slot, and the system provably converges to the latest commit **Status:** done

## Problem

ADR034 shipped "newest-wins per App: a newer revision **cancels the older active build** before starting." That preempts in-flight work, so whenever push cadence < build duration, every build is killed before finishing — a livelock with zero forward-progress guarantee, observed in production as endless cancel/redeploy cycles. Render and Vercel both supersede only **queued** work and let in-flight builds run to completion; bex's semantics are more aggressive and strictly worse under load.

## Fix (semantics)

1. **Run-to-completion:** an admitted, running build Job is never canceled by a newer generation.
2. **Latest-pending slot:** each App holds at most one pending generation; a newer push overwrites the slot (intermediate commits coalesce, never queue individually). When the running build finishes (success, failure, or timeout), the slot dispatches immediately.
3. **Point of no return:** once the image is pushed, that generation's rollout proceeds; the newer generation waits in the slot.
4. **Preserved exceptions:** the user's explicit Cancel verb (w2/m10) still preempts a running build — human intent, not supersede. A failed/timed-out build releases the slot immediately.

Guarantee: in any window of ~1 build duration, at least one deploy completes, and after pushes stop the App converges to the latest commit within one build cycle.

This lands **with ADR060 D1** (non-blocking build state machine, `Owns(Job)` watch) because the synchronous in-reconcile wait loop is exactly what makes "pending" awkward to represent today — doing them separately means touching the same code twice.

## Tasks (in order)

| id   | title                                                                    | est | depends_on | status        |
| ---- | ------------------------------------------------------------------------ | --- | ---------- | ------------- |
| t001 | ADR amendment: supersede applies to pending work, never in-flight        | 30m | —          | — **DONE**    |
| t002 | App status build-state model (externalize the wait-loop state)           | 45m | t001       | — **DONE**    |
| t003 | Non-blocking reconcile: `Owns(Job)` watch replaces the in-reconcile poll | 60m | t002       | — **DONE**    |
| t004 | Run-to-completion + latest-pending slot dispatch                         | 60m | t003       | — **DONE**    |
| t005 | Build SLIs: superseded-pending vs canceled-running + queue/run split     | 30m | t004       | — **DONE**    |
| t006 | Render parity: deploy lifecycle states across REST/GraphQL/MCP/dashboard | 30m | t005       | — **DONE**    |
| t007 | Simplify                                                                 | 20m | t006       | — **DONE**    |
| t008 | Test coverage: starvation regression + state-machine envtest             | 45m | t006       | — **DONE**    |
| t009 | Closeout                                                                 | 15m | t008       | — **DONE**    |

## Definition of done

- Under a simulated push cadence faster than build duration (envtest: push every ~60s against a ~5min build), every build cycle completes exactly one deploy, intermediate generations are coalesced (superseded-while-pending, no per-commit queue growth), and one cycle after pushes stop the App serves the latest commit.
- No code path cancels a running build Job on a newer generation; the only running-build cancellations are the explicit Cancel verb and the 30-minute `activeDeadlineSeconds`.
- `bex_build_*` series distinguish `superseded` (pending overwritten) from `canceled` (user verb) outcomes, and the queue-vs-run duration split is scrapeable.
- `make test` (operator envtest) green; deploy-status surfaces are consistent across REST/GraphQL/MCP and the dashboard, with any deliberate Render divergence recorded in docs/ADR018-render-parity.md.

## Source + Goal linkage

- **Source:** TPM discussion 2026-08-16 (production deploy starvation under frequent pushes: continuous cancel/redeploy, nothing completes) + [docs/ADR060-build-worker-reliability-and-performance.md](../../../docs/ADR060-build-worker-reliability-and-performance.md) D1/D5 + [docs/ADR034-scalable-build-pipeline.md](../../../docs/ADR034-scalable-build-pipeline.md) (the newest-wins-cancels-active line this milestone amends).
- **Goal linkage:** core Render-alternative reliability (push-to-deploy, ADR004/ADR017 pillar 4) — deploys must complete under real-world push cadence; also executes ADR060's highest-leverage deferral (D1).
- **Expected outcome:** forward-progress guarantee for builds; deploy success rate under rapid pushes goes from ~0 to one-per-build-cycle; supersede behavior aligns with Render/Vercel (queued work superseded, in-flight work completes).
- **Why now:** active production pain — the current semantics livelock under exactly the usage pattern (frequent pushes) the product is built for; and D1 is the prerequisite the rest of ADR060 (D2 classification, D6 honest admission) sequences behind.
- **Render parity included:** deploy lifecycle states (`queued`/`canceled`/superseded) are user-visible across REST/GraphQL/MCP/dashboard; Render's own supersede-queued behavior is the reference.
