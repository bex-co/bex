# w4 · m694 — A Blueprint's `numInstances` leaves autoscaling on, as Render's spec says

**Worker:** worker4 **Goal:** syncing a Blueprint that declares `numInstances` but no `scaling` block stores the manual count and leaves a service's autoscaling exactly as it was, as Render documents, instead of silently turning autoscaling off. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | `numInstances` without `scaling` stores the manual count and keeps autoscaling on — **DONE** | 45m | — |
| [t002](done/t002.md) | The disk refusal and ADR018 stop telling users that `numInstances` turns autoscaling off — **DONE** | 30m | t001 |
| [t003](done/t003.md) | Tests, and a dev-4 sync of an autoscaled service from a file that declares `numInstances` — **DONE** | 1h | t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 25m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004, t005 |
| [t007](done/t007.md) | Closeout — **DONE** | 10m | t006 |

Total: **7 tasks, about 3h35m**.

## Definition of done

- A Blueprint sync of a file that declares `numInstances` and no `scaling` on a service with autoscaling enabled keeps autoscaling enabled with its min, max and targets unchanged, stores the declared count as the manual instance count, and records no `autoscaling_config_changed` event. The same holds for validate's plan on REST, GraphQL, MCP and the dashboard.
- A file that declares `scaling` still sets autoscaling as before, and a service without autoscaling still takes `numInstances` as its running count.
- The disk refusal for an autoscaled service no longer advises declaring `numInstances`; it names a way that works (turning autoscaling off on the service).
- On dev-4, an autoscaled web service synced from such a file keeps autoscaling, and ADR018's services-lifecycle row describes the new rule.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect other than postgresql to w4`, 2026-11-26 (the aspect: scaling and autoscaling), item 1. Found by a research agent and re-checked by hand at `origin/main` `20dfa991d4` before filing; no open, blocked or done item owns it.
- **Render:** [render.com/docs/blueprint-spec](https://render.com/docs/blueprint-spec): "`numInstances` … _This value has no effect for services with autoscaling enabled._" and "If you add an existing service to a Blueprint, that service retains any existing autoscaling settings unless you add the `scaling` field in your Blueprint."
- **bex today:** `lego/backend/internal/apps/blueprint_plan.go:480-483` calls `turnOffAutoscaling(dst)` whenever the file applies `numInstances` without `scaling`, and the disk refusal at `:502-503` tells users to "declare numInstances in the file to turn autoscaling off". The rule came in without a Render citation; ADR018 (`docs/ADR018/services-lifecycle.md`, from w4/597) records it as plain behavior, with no user sign-off.
- **Not verified:** how Render lets a Blueprint turn autoscaling off (for example by removing a `scaling` block it used to declare); Render does not document it, so that path (`:467-469`) is left as it is.
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first).
- **Expected outcome:** a team that configured autoscaling in the dashboard can sync a Render-style Blueprint without losing it.
- **Why now:** the current rule silently changes a running service's scaling on every sync of a common file shape; w4/m695 changes the same Blueprint scaling code, so the two go together.
- **Render parity:** included: Blueprint validate and sync are served on REST, GraphQL, MCP and the dashboard.

## Record

Shipped 2026-11-27 by `/loopx w4`. Triage re-checked the premise at `06403db7c3`: `blueprint_plan.go:480-483` still called `turnOffAutoscaling`, and four tests pinned the old rule. Render's spec was re-read the same day and says `numInstances` "has no effect for services with autoscaling enabled" and an existing service "retains any existing autoscaling settings unless you add the scaling field".

- **t001.** A file that declares `numInstances` without `scaling` sets `spec.replicas` (the stored manual count) and leaves `spec.autoscaling` as it was. A sync that stores a count under autoscaling records neither `autoscaling_config_changed` nor `instance_count_changed`. That is the rule `Service.Scale` follows (w4/m364), now one helper, `recordInstanceCountChange`, shared by `Scale`, the store-backed `persistBlueprintIntent` and the store-less `recordStackSettingChanges`.
- **t002.** Adding a disk to an autoscaled service is refused as "a service with a disk cannot use autoscaling; turn autoscaling off on the service first". Validate used to locate this refusal by matching words in its sentence: it pointed at `numInstances`, and after the rewording would have pointed at `scaling`, neither of which the file declared. It now locates every service field conflict at the field it names when the file declares that field (`blueprintConflictField`), as ADR082 states. So the refusals now sit at `services[i].disk`, at `.plan` for the free plan beside a disk, at `.numInstances` for two instances, and at `.disk.sizeGB` for a shrink. The ADR018 autoscaling row in `docs/ADR018/services-lifecycle.md` and the Blueprint table in `docs/render-artifacts/service-events.md` describe the new rule.
- **t003, dev-4.** bex-api was rebuilt from this tree (`dev-env.sh 4 up`), with a disposable verified identity and its Pro workspace (disposable). The run deployed an image web service `m694-walk-<hex>` (disposable) through `POST /v1/blueprints/deploy`, waited for it to go live, and turned on autoscaling with `PUT …/autoscaling` (min 1, max 3, CPU 70%). Then it validated and synced the same file with `numInstances: 2` and no `scaling`. Validate answered `valid: true` with one `update` whose `changedFields` were exactly `[numInstances]`. After the sync, `GET /v1/services/{id}` read `numInstances: 2` with `autoscaling {enabled: true, min: 1, max: 3, cpu 70 on, memory off}`, as before the sync, and `GET …/autoscaling` matched. The events feed held one `autoscaling_config_changed`, the `PUT`'s, and no `instance_count_changed`. The sync opened one `blueprint_sync` deploy, which matches Render's "Render automatically redeploys any affected services" (render.com/docs/infrastructure-as-code). The service's Deployment kept `spec.replicas: 1`, the autoscaler's count. Validating `numInstances: 1` plus a disk answered the new refusal at `services[0].disk`. The service and the account were then deleted (`DELETE /v1/users`, the identity lookup empty, the namespace terminating).
- **t004.** Render parity holds. REST, GraphQL and MCP validate share the planner, and `TestBlueprintNumInstancesLeavesAutoscalingOnAcrossSurfaces` pins the `[numInstances]` plan and the disk refusal on each. The dashboard renders GraphQL's `changedFields` and the refusal unchanged. No drift was found; how a Render Blueprint turns autoscaling off stays unverified and out of scope.
- **t005.** `/simplify`: the instance-count event rule repeated in three places became `recordInstanceCountChange`, and the scaling comment was tightened. There were no efficiency findings.
- **t006.** Each guarded line was reverted in turn and failed its test. Restoring `turnOffAutoscaling` failed 5 tests (plan, store-less events, managed events, cross-surface, disk rules). Each event gate (store-backed, store-less, the shared helper) failed its own events test, and the helper's also failed `TestScaleUnderAutoscalingStoresTheCountWithoutAnEvent`. The old advice failed 2 tests, and dropping the conflict location failed 2. The real-Postgres `TestPGBlueprintSyncSettingChangesListInTheServiceFeed` passed against the pinned deps. It now turns autoscaling off with `DeleteAutoscaling` before it adds the disk.
