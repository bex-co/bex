# w7 · m84 — Operator reconcile convergence: no `CreateOrUpdate` may PUT on a no-op

**Worker:** worker7 **Goal:** operator write load becomes proportional to actual change instead of to App count × time. **Status:** done

## Tasks (in order)

| id   | title                                                                       | est | depends_on               |
| ---- | --------------------------------------------------------------------------- | --- | ------------------------ |
| t001 | Reproduce: a two-reconcile envtest that asserts owned objects do not change — **DONE**   | 40m | —                        |
| t002 | Fix the Deployment projection so it matches the server-defaulted object — **DONE**       | 45m | w7/m84/t001              |
| t003 | Sweep the remaining `CreateOrUpdate` sites across the three controllers — **DONE**       | 50m | w7/m84/t002              |
| t004 | Promote the two-reconcile assertion to a shared invariant over every owner — **DONE**    | 40m | w7/m84/t003              |
| t005 | Simplify the code this milestone changed — **DONE**                                      | 30m | w7/m84/t004              |
| t006 | Test coverage for the shipped behavior — **DONE**                                        | 40m | w7/m84/t004              |
| t007 | Closeout — **DONE**                                                                      | 15m | w7/m84/t005, w7/m84/t006 |

## Definition of done

A second reconcile of an unchanged App, Database or KeyValue issues **zero writes** — every owned object's `resourceVersion` is stable and `controllerutil.CreateOrUpdate` returns `OperationResultNone` — asserted by an invariant covering all three controllers that turns red if a future projection drifts from the server defaults. No pod rollout is triggered on existing production Deployments by the change.

## Source + Goal linkage

- **Source:** [`.pm/w7/builder-issues.md`](../builder-issues.md) §3.6 (P5), generalizing **[`.pm/w7/028.md`](../done/028.md)** (filed 2026-08-17 by `m81`'s `/simplify` pass). `w9/m57` is the single-field precedent — it fixed exactly this mechanism for Services (`Protocol`) without generalizing it.
- **Goal linkage:** [`.pm/GOAL.md`](../../GOAL.md) #6 and #2 — this is perpetual per-App apiserver + etcd write load, and etcd write capacity is the scarcest resource on a 3-node control plane.
- **Expected outcome:** the self-sustaining reconcile loop stops; operator and apiserver load track real change. Nothing user-visible changes, which is precisely why nothing has ever reported it.
- **Why now:** it is silent, it scales linearly with tenant count, and it was just discovered with a precise fix sketch and a known-good precedent — the cheapest it will ever be to fix. `app_controller.go:3398` watches Deployments on `ResourceVersionChangedPredicate`, so the operator's own PUT feeds straight back into its queue; `deployment_projection.go:296` rebuilds `Containers` wholesale and `:300` forces `TerminationGracePeriodSeconds`, so the mutated object can never `DeepEqual` the defaulted one. There is currently **no `OperationResultNone` assertion anywhere** in the operator, so nothing would catch a recurrence.
- **Render parity task omitted:** yes — this is an operator-internal reconcile mechanism. No REST, GraphQL, MCP or dashboard surface changes; pods do not even roll (the template hash is computed post-defaulting), so there is nothing for a user-facing surface to reflect.

## Outcome — **DONE 2026-08-19**

A converged reconcile of an App, KeyValue or Database now issues **zero writes**, proven by
an invariant that counts the client's requests rather than watching `resourceVersion` — and
that distinction is the milestone's main finding.

**`028`'s chain was right about the PUT and wrong about the loop.** The reproduction (t001)
measured **no `resourceVersion` movement anywhere**: the API server re-defaults an incoming
object, finds it byte-identical to what is stored, and skips etcd, so the redundant PUT
fires no watch event and the self-sustaining `ResourceVersionChangedPredicate` requeue loop
never actually closed. An RV-based test — the obvious one to write, and the one `028`'s fix
sketch proposed — would have passed against the very bug it was written for. What is real is
one full decode/validate/admission round trip per object per pass, on the API server's
mutating path, scaling with tenant count.

**The blind deletes were the larger half by count, and nobody had predicted them.** Every
App issued **three** DELETEs per pass for Traefik middlewares it does not have, against one
PUT; KeyValue and Database each issued two or three more. They now check existence first —
but the t005 efficiency review corrected what that is worth, and the corrected number is
the honest one: only **one of the six** (the KeyValue backup CronJob, a typed kind the
controller already watches) actually loses its round trip. The rest are unstructured or
ride the deliberately uncached secret client, and controller-runtime does not serve either
from cache, so there the fix converts a write into a read — off the API server's mutating
path and out of the write audit stream — rather than removing a request.

**Rollout safety is proven, not argued** — and by the same fact: if the stored bytes never
changed, the pod-template hash computed from them cannot change. `server_defaults_envtest_test.go`
stores a Deployment from the pre-fix projection, applies today's, and asserts the stored pod
template is `Equal` and the `resourceVersion` does not even move.

Fixed: the Deployment, CronJob (+ manual-run Job), Valkey StatefulSet and KeyValue backup
CronJob projections via one shared `server_defaults.go`; the KeyValue Service's port
`Protocol` (w9/m57's fix, on the object it did not reach); an empty annotations map the CNPG
Cluster projection wrote on every pass; and six blind deletes (see above for what that
last one is actually worth). `CreateOrPatch` and
server-side apply were both evaluated and rejected in writing — the first diffs the same
in-memory object before and after the mutate, so it changes the verb rather than the request
count; the second sends a request every pass by construction.

**Exemptions: one, named** — the owner's own `status` subresource, which several terminals
write unconditionally. Out of this DoD, and envtest cannot judge it honestly (no kubelet, so
a rollout never settles); measured and filed as [`033`](../033.md). Also filed:
[`032`](../032.md) (the CNPG Cluster may still churn against a live CNPG webhook — envtest
uses a stub CRD, so this repo cannot see it), [`034`](../034.md) (cron containers pull
`IfNotPresent` where the Deployment path deliberately pulls `Always`), [`035`](../035.md)
(a pre-existing envtest ordering coupling, verified against a stashed tree).

The durable half is `reconcile_convergence_envtest_test.go`: one helper over eight shapes
across all three controllers, each asserting the kinds it must own so a shape cannot pass
vacuously, plus an anti-tautology spec that re-runs the reconciler's own `CreateOrUpdate`
with the pre-fix projection six times — one default family dropped each time — and proves
the recorder sees the PUT every time.
