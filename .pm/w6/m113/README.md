# w6 · m113 — A failed config-change deploy marks the whole service `Failed` while the previous release keeps serving 200

**Worker:** worker6 **Goal:** `phase` stops calling a service failed while it is healthily serving traffic — a failed rollout attributed to the deploy that failed, not to the release still answering requests. **Status:** todo

## Background (found live, 2026-08-27, 25th `/qa-find-bugs` run)

Ran journey 4 (env vars / secret files) on `srv-da7o6ovvqdcc73bpn9hg`, a Free-tier web service that was `Running` with `Latest deploy: Live`. Added one secret file through **Environment → Edit → Add secret file → Save and deploy**. The save succeeded and the file persisted (`GET /v1/services/<id>/secret-files` → `[{"secretFile":{"name":"qa-20260827-secret.txt"}}]`), and it opened a `config_change` deploy, which failed:

```
GET /v1/services/srv-da7o6ovvqdcc73bpn9hg/deploys/dep-da7r98nkrsvc73c3m5mg
{ "status": "build_failed", "trigger": "config_change",
  "createdAt": "2026-08-27T03:53:06.760116Z", "finishedAt": "2026-08-27T03:53:15.734821Z",
  "failureReason": "build failed: PodFailurePolicy: Container clone for pod
    bex-build/bld-tea-d98210cbbpdc73dcrkvg-qa-20260826-webhook-svc-gen-7q7d5p
    failed with exit code 90 matching FailJob rule at index 1" }
```

That much is a build failing, which is allowed to happen. What follows is the defect. **All three surfaces then reported the whole service as failed, while the service kept serving:**

```
GET /v1/services/srv-da7o6ovvqdcc73bpn9hg      → "phase": "Failed", "replicas": 1,
                                                  "urls": ["https://qa-20260826-webhook-svc.onbex.co"]
query { service(id:"srv-da7o6ovvqdcc73bpn9hg") { phase replicas revision suspended } }
                                               → { "phase": "Failed", "replicas": 1,
                                                   "revision": "rev-4", "suspended": "not_suspended" }
dashboard header                               → "Service  Failed   Latest deploy  Build Failed"
```

and, from outside the browser at the same time — twice, and again after a second failed deploy:

```
$ curl -sS -o /dev/null -w "status=%{http_code} time=%{time_total}s\n" \
    -A "Mozilla/5.0 … Chrome/140" https://qa-20260826-webhook-svc.onbex.co/
status=200 time=0.576169s
status=200 time=0.566402s
status=200          # after dep-da7rag7vqdcc73bpnin0 also failed
```

The previous release (`dep-da7oat7krsvc73c3lsr0`, `live`) is still up and answering. The new rollout's pod never became ready — the operator recorded that honestly as its own event, `server_failed` with `details.reasonCode: "readiness_failed"` at `03:53:58` — and Kubernetes correctly kept the old ReplicaSet serving. Only the coarse `phase` field escalated a failed *rollout* into a failed *service*.

## Root cause

`lego/operator/internal/controller/app_controller.go:3912-3913`:

```go
func (r *AppReconciler) fail(ctx context.Context, app *appv1alpha1.App, reason string, err error) (ctrl.Result, error) {
	app.Status.Phase = appv1alpha1.PhaseFailed
```

`r.fail` sets `PhaseFailed` unconditionally, with no consideration of whether an earlier release is still serving.

**The codebase already wrote down the correct invariant — for the sibling case.** `lego/types/v1alpha1/app_types.go:1104-1112`, added by `w6/m52`:

```go
// PhaseCanceled: the user canceled the release that was rolling and no
// earlier release ever succeeded, so nothing is running and nothing failed.
// Reusing PhaseFailed here made a service contradict its own deploy history
// … (w6/m52). A cancel with an earlier healthy release never reaches this
// state: that release keeps serving and the phase stays Running.
```

That last sentence is exactly the property violated here, with *build failure* substituted for *cancel*. And the guard that enforces it for cancel is a single line — `app_controller.go:577`, `if app.Status.Image != "" { return r.dispatchRuntime(...) }` — the **only** place in the file that reasons about a surviving release before settling a terminal phase (grep for `Status.Image != ""` in that file returns 1 hit).

## Not a duplicate of `w6/m52` — and its control case is why this survived

`w6/m52` (done) fixed cancel-with-no-prior-release and its DoD explicitly locked in the opposite for build errors:

> "A service reaching `PhaseFailed` through **a genuine build error** … still shows 'Failed' — verified live as the explicit control case, not assumed."

That control case was a **first** deploy — no prior release existed, so `Failed` was correct there. m52 never exercised a build error *with* a healthy release behind it, which is this case. So this is the untested quadrant of m52's own matrix, not a regression of it.

## Target behavior — a decision, not a parity lookup

`w6/m52`'s closing note records the parity fact that makes this a design question rather than a lookup: **"Render has no service-level status at all (its header renders the latest deploy), which is exactly why bex's `phase` — a documented bex-only extra — was free to drift."** There is no Render behavior to copy. The milestone must therefore *choose* and write down what `phase` means when the newest rollout failed and an older release is serving, with the two defensible answers being:

- **`Running`** — `phase` describes what is serving; the failed attempt is fully described by `deploy.status` + the `Latest deploy` badge the header already renders beside it. This matches the invariant `app_types.go:1104-1112` already states for cancel.
- **A distinct value** (e.g. `Degraded`) — `phase` describes the service *and* flags that the newest attempt did not land. Costs a CRD enum addition (`+kubebuilder:validation:Enum=Pending;Building;Deploying;Running;Hibernated;Canceled;Failed`, `app_types.go:1094`) and a projection change in `dashboard/src/features/services/lib/status.ts`, which today carries only the raw phase.

Whichever is chosen, "a service answering 200 reports `Failed`" must not be the answer.

## Blast radius (counted, not estimated)

`grep -rn "r\.fail(" lego/operator/internal/controller/*.go | grep -v _test` → **47** call sites, every one of which inherits the unconditional `PhaseFailed`. They are not interchangeable: `BadSpec` and `ProtectedSecretReference` fire before any release could exist, while `ReasonBuildFailed`, `DeployFailed`, `IngressFailed`, `ServiceFailed` and `NetworkPolicyFailed` can all fire with a healthy release already serving. The fix must be decided per class, not applied blindly to all 47 — and the call sites that report `Failed` correctly today do so **because** no release exists, so they need regression tests, not just the broken path.

## Adjacent classes

`phase` already distinguishes `Canceled` (user stopped it, nothing running) from `Failed` (something broke). This milestone adds a third distinction — *something broke but the service is still up* — and must place all of them, including `Hibernated` (free-tier sleep, nothing running **by design**) so a sleeping service is never folded into whatever new value is chosen. `suspended` stays orthogonal: it read `not_suspended` throughout.

## Unverified (carried forward)

- Whether the same escalation happens for the other four App-typed kinds (private · worker · cron · static). One shared reconcile path argues yes; not exercised this run.
- Whether `phase` recovers on its own once a later deploy succeeds, or stays `Failed` until something else moves it — the fixture's builds kept failing, so a successful recovery deploy was never observed.
- **The build failure itself is a separate, unfiled claim.** `Container clone … exit code 90` reproduced 2/2 on consecutive `config_change` deploys for `bex-co/bex-hello-go-live`, a repo that built successfully at `00:22` and `00:31` the same day. It is **not** a platform outage — the control is clean: `beancount-cms-v2` had a `build_in_progress` at `03:50` and a `live` at `03:28`, `block-eden-mono` live at `01:32`, `eden-cms-v2` live at `01:51`, all in the same window. Cause unverified (no clone-container logs were retrievable: the `logs` query for the build window returned `[]`), and it may belong to the open `w6/m97` / `w6/m99` clone-credential family. It is the *trigger* for this milestone's repro, not its subject.

## Tasks (in order)

| id   | title                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Decide what `phase` means when a rollout fails over a serving release, and write it into the CRD | 40m | —          |
| t002 | Classify all 47 `r.fail` call sites into can-have-a-serving-release or cannot     | 45m | t001       |
| t003 | Apply the decision, guarding only the classes t002 identified                     | 45m | t002       |
| t004 | Render parity — `phase` across REST/GraphQL/MCP and the dashboard header          | 30m | t003       |
| t005 | Simplify — `/simplify` over the code this milestone changed                       | 20m | t004       |
| t006 | Test coverage                                                                     | 45m | t004       |
| t007 | Closeout                                                                          | 15m | t006       |

## Definition of done

1. On a service with a healthy live release, cause a rollout to fail (an env-var or secret-file save whose build fails is the repro used here). While `curl -sS -o /dev/null -w "%{http_code}"` against the service URL returns **200**, `GET /v1/services/<id>` does **not** report `phase: "Failed"` — it reports whatever t001 chose, and that choice is written into `app_types.go`'s enum comment.
2. REST, GraphQL and the dashboard header all report that same value for the same service at the same moment — the three agreed on `Failed` in the filing capture, so they must agree on the new value too.
3. The header still shows the failed attempt: `Latest deploy: Build Failed` remains, and the deploy row's own `status` is still `build_failed`. The fix must not hide the failure, only stop attributing it to the service.
4. `w6/m52`'s control case still holds: a build error on a **first** deploy, with no prior release, still reports `Failed`. Re-run it rather than assuming.
5. A canceled deploy with a prior healthy release still reports `Running`, per `app_types.go:1104-1112` — the invariant this milestone generalizes.
6. A free-tier service asleep reports `Hibernated`, not the new value.
7. t002's classification of all 47 call sites is recorded in this README, with the count and the per-class decision.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 25th run, 2026-08-27, journey 4 (env vars / secret files). Probes and complete responses are pasted inline; the `curl` lines are the durable artifact for the "still serving" half and can be re-run by anyone. `.playwright-mcp/` captures are gitignored and session-local and nothing here rests on them. The secret file created for the repro was deleted (`GET …/secret-files` → `[]`).
- **Goal linkage:** `docs/ADR004-app-deployment.md` owns the deploy/phase lifecycle and `docs/ADR018-render-parity.md` records `phase` as a bex-only extra beyond Render's deploy-status-only header. ADR008's hosting pillar: a status field that calls a serving service failed is worse than no status field.
- **Expected outcome:** a failed rollout stops paging the operator about a service that never stopped serving, and `phase` becomes a field worth trusting.
- **Why now:** the trigger is an ordinary user action — saving an env var or secret file — and the escalation is silent and total: three surfaces agree with each other and all three are wrong. `w6/m52` already paid for this lesson once in the cancel path and wrote the invariant down; the build path never got it.
- **Render parity task included:** yes — `phase` is exposed over REST and GraphQL and drives the dashboard header, and t001 may add a CRD enum value. Render has no service-level status, so t004 records the divergence deliberately rather than resolving it by lookup.
