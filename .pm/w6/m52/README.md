# w6 · m52 — Canceling a first-ever deploy misreports the service as Failed, not Canceled

**Worker:** worker6 **Goal:** a service's own status always agrees with its deploy history — a user-initiated cancel never reads as an error **Status:** todo

## Tasks (in order)

| id   | title                                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Decide and name the target behavior for a canceled first-ever deploy (check Render parity first) | 20m | —          |
| t002 | Add the CRD/type-level signal needed to carry the distinction through to GraphQL                 | 30m | t001       |
| t003 | Update `settleCanceledRelease` to set the new signal instead of reusing `PhaseFailed`             | 30m | t002       |
| t004 | Update the dashboard's status derivation/labels to render the new state distinctly (en + zh)      | 30m | t002       |
| t005 | Render parity: REST/GraphQL/MCP/UI agree on the new status; render.com behavior confirmed          | 20m | t003, t004 |
| t006 | Simplify                                                                                          | 15m | t005       |
| t007 | Test coverage                                                                                     | 30m | t006       |
| t008 | Closeout                                                                                          | 10m | t007       |

## Definition of done

- Canceling a first-ever deploy on a fresh web service no longer shows "Failed" in the service header; it shows the new distinct status, and this agrees with `deploy(serviceId:, deployId:).status` (`"canceled"`) via a raw GraphQL query the same way this hunt confirmed the bug (`server(id:).phase` currently returns `"Failed"` for this exact case).
- A service reaching `PhaseFailed` through a genuine build error (a real crash/nonzero exit, or any deploy where the operator sets Failed via an actual error path, not a user cancel) still shows "Failed" — verified live as the explicit control case, not assumed.
- The distinction is verified live for at least two of the five App-typed service kinds (web service and cron job) — the mechanism is one shared reconcile path, so this is a spot-check of the fix's reach, not five separate investigations.
- Render-parity closing task applies: this changes a GraphQL-exposed field's semantics and the dashboard UI that reads it (`docs/ADR018-render-parity.md` ledger).

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co` hosting surfaces, 2026-08-24 (run from a `/loop 30m` session, testing Cancel during the earliest possible deploy phase — clone/setup, before the build step starts — a narrow timing window not covered by earlier iterations' general cancel/rollback testing).
- **Goal linkage:** ADR006 (bex-api Render parity) and the platform's basic trust contract — a resource's own status must never contradict its own history. A user who deliberately cancels an action and then sees a red "Failed" badge has no way to tell, from the UI alone, whether they broke something or the platform did.
- **Expected outcome:** the service-level phase and the deploy-level status agree in every case; "Failed" means an actual failure occurred, never a successful user-initiated cancel.
- **Why now:** cheap, well-scoped fix with the exact root cause already pinned to file:line — the risk is this silently confuses every user who ever cancels a service's very first deploy, which is a common, ordinary interaction (change your mind right after clicking Deploy), not an edge case.

## Live evidence (2026-08-24, workspace `bex`, service `qa-20260824-cancel` / `srv-da6dsna3bj6s73aie8k0`, deploy `dep-da6dsna3bj6s73aie8kg`)

**Repro steps:**

1. Created a fresh `qa-20260824-cancel` web service from `bex-co/frontend-onefx-boilerplate`. Landed directly on its first deploy's detail page, status `Created`, log line `==> Build queued` — the earliest observable phase.
2. Clicked **Cancel** immediately, confirmed through the "Cancel this deploy? The in-progress deploy will be stopped. The last successful deploy remains live." dialog.
3. The deploy detail page itself updated correctly and consistently: **Deploy status: Canceled**, status timeline shows `Deploy created` → `Canceled`, log line `==> Deploy canceled`. No inconsistency here.
4. The **service header's status pill**, however, first showed **Building** (a few seconds of expected async lag), then settled on **Failed** — and stayed there. It never converged to anything reflecting "canceled."
5. Confirmed this is not a UI-only staleness/caching artifact: queried `https://api.bex.co/graphql` directly from the page —
   ```graphql
   query {
     server(id: "srv-da6dsna3bj6s73aie8k0") { id phase }
     deploy(serviceId: "srv-da6dsna3bj6s73aie8k0", deployId: "dep-da6dsna3bj6s73aie8kg") { id status }
   }
   ```
   Response: `{"deploy":{"id":"dep-da6dsna3bj6s73aie8kg","status":"canceled"},"server":{"id":"srv-da6dsna3bj6s73aie8k0","phase":"Failed"}}` — the backend itself disagrees with its own deploy record.

**Root cause:** `lego/operator/internal/controller/app_controller.go:552-573` (`settleCanceledRelease`) is the operator's handler for "a generation whose build artifact the backend already deleted" (i.e., a user-canceled build). When no prior successful release exists yet (`app.Status.Image == ""` — true for any first-ever deploy), it sets:

```go
app.Status.Phase = appv1alpha1.PhaseFailed
...
meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
    Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: "BuildCanceled",
    Message: "build canceled before a release became available", ObservedGeneration: app.Generation,
})
```

The Condition's `Reason`/`Message` correctly say `BuildCanceled` / "build canceled before a release became available" — the operator *knows* this was a cancel, not a failure. But the coarse `Phase` enum (`appv1alpha1.PhaseFailed`) has no distinct value for "user canceled, no release exists yet," so it reuses the same `PhaseFailed` a genuine build error produces. Downstream, nothing carries the Condition Reason forward: `dashboard/src/features/services/lib/status.ts`'s `toServiceView` projects only the raw `phase` string onto `ServiceView` (never a Condition/Reason field), and `PHASE_STATUS` (same file, line ~189) maps `"failed"` unconditionally to `{ key: "failed", variant: "destructive" }` → the dashboard's `STATUS_LABEL` (`lib/labels.ts:16`) renders `services.statusFailed` = "Failed". There is no signal anywhere in this chain that could disambiguate even if a downstream layer tried to.

**Blast radius:** single call site (`app_controller.go:510` → `settleCanceledRelease`), reached uniformly by the generic App reconcile loop — so this affects all 5 App-typed service kinds (web service, private service, background worker, cron job, static site) whenever their very first deploy is canceled before any image is produced, not a per-type bug.

**Adjacent classes this fix must not blur:** a service whose `PhaseFailed` comes from an actual build/deploy error (with or without a prior successful release) must keep showing "Failed" — t007's regression tests must cover this control case explicitly, not just the canceled-first-deploy case.
