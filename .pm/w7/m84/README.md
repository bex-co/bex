# w7 · m84 — Operator reconcile convergence: no `CreateOrUpdate` may PUT on a no-op

**Worker:** worker7 **Goal:** operator write load becomes proportional to actual change instead of to App count × time. **Status:** todo

## Tasks (in order)

| id   | title                                                                       | est | depends_on               |
| ---- | --------------------------------------------------------------------------- | --- | ------------------------ |
| t001 | Reproduce: a two-reconcile envtest that asserts owned objects do not change   | 40m | —                        |
| t002 | Fix the Deployment projection so it matches the server-defaulted object       | 45m | w7/m84/t001              |
| t003 | Sweep the remaining `CreateOrUpdate` sites across the three controllers       | 50m | w7/m84/t002              |
| t004 | Promote the two-reconcile assertion to a shared invariant over every owner    | 40m | w7/m84/t003              |
| t005 | Simplify the code this milestone changed                                      | 30m | w7/m84/t004              |
| t006 | Test coverage for the shipped behavior                                        | 40m | w7/m84/t004              |
| t007 | Closeout                                                                      | 15m | w7/m84/t005, w7/m84/t006 |

## Definition of done

A second reconcile of an unchanged App, Database or KeyValue issues **zero writes** — every owned object's `resourceVersion` is stable and `controllerutil.CreateOrUpdate` returns `OperationResultNone` — asserted by an invariant covering all three controllers that turns red if a future projection drifts from the server defaults. No pod rollout is triggered on existing production Deployments by the change.

## Source + Goal linkage

- **Source:** [`.pm/w7/builder-issues.md`](../builder-issues.md) §3.6 (P5), generalizing **[`.pm/w7/028.md`](../done/028.md)** (filed 2026-08-17 by `m81`'s `/simplify` pass). `w9/m57` is the single-field precedent — it fixed exactly this mechanism for Services (`Protocol`) without generalizing it.
- **Goal linkage:** [`.pm/GOAL.md`](../../GOAL.md) #6 and #2 — this is perpetual per-App apiserver + etcd write load, and etcd write capacity is the scarcest resource on a 3-node control plane.
- **Expected outcome:** the self-sustaining reconcile loop stops; operator and apiserver load track real change. Nothing user-visible changes, which is precisely why nothing has ever reported it.
- **Why now:** it is silent, it scales linearly with tenant count, and it was just discovered with a precise fix sketch and a known-good precedent — the cheapest it will ever be to fix. `app_controller.go:3398` watches Deployments on `ResourceVersionChangedPredicate`, so the operator's own PUT feeds straight back into its queue; `deployment_projection.go:296` rebuilds `Containers` wholesale and `:300` forces `TerminationGracePeriodSeconds`, so the mutated object can never `DeepEqual` the defaulted one. There is currently **no `OperationResultNone` assertion anywhere** in the operator, so nothing would catch a recurrence.
- **Render parity task omitted:** yes — this is an operator-internal reconcile mechanism. No REST, GraphQL, MCP or dashboard surface changes; pods do not even roll (the template hash is computed post-defaulting), so there is nothing for a user-facing surface to reflect.
