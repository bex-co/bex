# w5 · m113 — Availability failure edges carry the Ready transition time (re-scopes w4/200)

**Worker:** worker5 **Goal:** `server_failed` and its datastore twins are stamped at the Ready transition even when the debounced first unhealthy pass moved the checkpoint, so outages aren't understated. **Status:** done — 2026-10-05

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Replay the reconciler's real observation sequence in the PG test — **DONE** | 30m | — |
| t002 | Bound service failure edges by the last availability change — **DONE** | 45m | t001 |
| t003 | Apply the same bound to datastores — **DONE** | 30m | t002 |
| t004 | Replay w4/200's probe live and settle w4/200 — **DONE** | 30m | t003 |
| t005 | Render parity — **DONE** | 15m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

## Definition of done

- A store test replaying the reconciler's sequence (healthy → debounced pass with phase Deploying → unhealthy) stamps `server_failed` at the Ready transition; the datastore twin does the same.
- The recovery edge and the anti-flap debounce are unchanged.
- A live replay of w4/200's probe shows `server_failed` within ~10 s of the first 503, and w4/200 is closed or re-scoped with that evidence.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05, of `44a612e8d` (w4/196), plus w4/200 filed the same day, which attributes the remaining ~40 s to operator Ready lag (cause marked unverified). Code-verified mechanism: the debounced first unhealthy pass still records the phase change, which moves the checkpoint's `updated_at` past the Ready transition. `availabilityEdgeAt` then falls back to the next pass's observation time, one 30 s resync late, matching the ~38–40 s measured before and after w4/196. The recovery edge has no debounce and is exact, as observed.
- **Goal linkage:** ADR010 observability and ADR052 notifications: truthful incident timing.
- **Expected outcome:** The Events feed, webhooks and push report when an outage started to within one probe period.
- **Why now:** Keeps w4/200's operator diagnosis from chasing the wrong component, and finishes a fix shipped on 2026-10-04.
- **Render parity included:** event timestamps change on every surface.
- **Re-scopes w4/200:** a pointer was added to `w4/200.md`; t004 settles it.

## Evidence — 2026-10-05

**Cause, confirmed.** On a readiness loss the operator writes phase Deploying and Ready=False in one status write. The reconciler's first unhealthy pass is debounced: availability is unseen, but the phase change is still recorded. That write moved the checkpoint's `updated_at`, the floor `availabilityEdgeAt` orders an edge after, past the Ready transition. The confirming pass's edge then fell back to its own observation time, a resync late.

**Fix.** `AvailabilitySuppressed` on `ObservedServiceState`/`ObservedDatastoreState`, set by the two guards' shared suppress helpers. A suppressed pass still records its phase but leaves `updated_at` alone. Every other change still moves the floor.

- The task's two candidate floors (`healthy_transition_at`, or a new `availability_changed_at`) were rejected. Both would date a crash during a rollout from the rollout start: Ready flips False when a rollout starts, and a later CrashLoopBackOff keeps that transition time.
- The production guard chain is now `guardServiceObservation`/`guardDatastoreObservation`, so the replay tests run the exact production composition.
- A pass that is both suppressed and observed is refused.

**Tests (t001, t003, t007).**

- `TestPGFailureEdgeOutlivesTheDebouncedPhaseWrite` and its datastore twin replay the real sequence (healthy → debounced Deploying/Provisioning pass → confirm) against Postgres 17.
- Both are red without the fix: the edge lands at `:95`, the confirming pass, instead of `:62`.
- The service test's second half catches the over-broad variant (holding the floor on every unobserved pass), which would date a crash at the `:198` rollout start.
- The recovery edges and the stale-transition guard are unchanged; the existing tests pass.
- Store package green on fresh dependencies, and `make lint` passes.

**Live replay (t004), dev-5, a bex-api built from this tree.** Owned Free `traefik/whoami` (`srv-db22qu1jg4r9qfb28kq0`), health check `/health`, readiness 3×10 s, liveness 6×10 s. Flipped with `POST /health 500` through a pod port-forward.

| Run | Build | Flip | Pod Ready=False (traffic stops) | `server_failed.at` | Recorded at |
| --- | --- | --- | --- | --- | --- |
| 1 | pre-refactor | 23:13:54 | 23:14:23 | **23:14:23** | 23:14:53.97 |
| 3 | final | 23:26:56 | 23:27:23 | **23:27:23** | 23:28:21.97 (a debounced pass at 23:27:51.97 recorded Running → Deploying) |

Both edges land 0 s after the first failed request. Before this fix, run 3 would have been stamped 23:28:21.97, the w4/200 lag.

- Runs 2 and 4 crossed the liveness restart at flip + 60 s. In run 2 the restarted pod was briefly Ready, so Ready flipped again. The anti-flap debounce folded that into one edge, at the second flip.
- Recovery edges matched the pod's Ready transitions.

**Cleanup.** The fixture service is deleted (`DELETE` 204, `GET` 404). Its App CR is still waiting for the shared local operator's finalizer, because the mock cluster has no `bex-build` namespace, so finalization stalls for every dev-N deletion. Filed as w5/084, with no manual finalizer edit, which would orphan the copied registry Secrets.

**Render parity (t005).** Every surface carries the fact's `at`:

- REST, GraphQL and MCP through the Events feed (`store/events.go`);
- webhooks and push through `ListWebhookEvents`, which use `recorded_at` as the delivery cursor, so a backdated `at` can't slip behind a watermark.

No shape change. Render's `server_failed` timestamp is when the instance failed, which is what bex now reports.

**`/simplify` (t006), applied:**

- the shared guard methods;
- `openDatastoreTestStore` and the existing CR fixtures in the tests (the inline prologue had closed the pool before the tenant cleanup ran, leaking the tenant);
- the suppressed-and-observed validation;
- field docs explaining why the store can't infer the flag;
- comments state the rule.

Efficiency review: no findings. The CASE sits inside the `IS DISTINCT FROM`-guarded UPDATE, and `updated_at` is unindexed, so HOT updates still apply.

Follow-up filed: w5/083. When the first pass to see a rollout is already the debounced crash, the edge is dated from the rollout start. That's right for Recreate rollouts and early for a RollingUpdate whose old pods died later. The fix is to back that branch with the `Serving` transition.

