# w4 · m695 — A Blueprint `scaling` block works on a background worker, and export round-trips it

**Worker:** worker4 **Goal:** a Blueprint can declare autoscaling for a background worker, as Render's spec allows, and exporting an autoscaled worker writes its `scaling` block instead of flattening it to a fixed count. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Validate and sync accept `scaling` on a background worker — **DONE** | 45m | — |
| [t002](done/t002.md) | Export writes an autoscaled worker's `scaling` block — **DONE** | 30m | t001 |
| [t003](done/t003.md) | dev-4: a worker Blueprint with `scaling` validates, syncs and exports unchanged — **DONE** | 30m | t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 25m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004, t005 |
| [t007](done/t007.md) | Closeout — **DONE** | 10m | t006 |

Total: **7 tasks, about 3h05m**.

## Definition of done

- Blueprint validate and sync accept a `scaling` block on a `type: worker` service on every surface and turn on its autoscaling with the declared range and targets.
- Generating a Blueprint from an autoscaled worker writes `scaling` (min, max, targets), not `numInstances: <max>`, and the generated file validates.
- On dev-4, a worker Blueprint with `scaling` syncs, and its export matches the file.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect other than postgresql to w4`, 2026-11-26 (the aspect: scaling and autoscaling), item 2. Re-checked by hand at `origin/main` `20dfa991d4` before filing; no open, blocked or done item owns it.
- **Render:** [render.com/docs/blueprint-spec](https://render.com/docs/blueprint-spec) § Scaling: "These fields apply scaling settings for the following service types: Web services, Private services, Background workers." The pinned schema (`lego/backend/internal/apps/schema/render.yaml.json`) puts `scaling` on `serverService`, whose types include `worker`.
- **bex today:** validation refuses it (`lego/backend/internal/apps/deploy.go:3460-3461`, "scaling is not available on background workers"), and export writes an autoscaled worker as `numInstances: <max>` (`blueprint_generate.go:467-485`, pinned by `blueprint_generate_test.go:248-256`). The API already supports worker autoscaling (w4/m101).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first).
- **Expected outcome:** a Render Blueprint with an autoscaled worker deploys on bex, and an export can be synced back without losing the worker's autoscaling.
- **Why now:** a Render-style file fails validation with 400 today; it shares the Blueprint scaling code with w4/m694.
- **Render parity:** included: Blueprint validate, sync and export are served on REST, GraphQL, MCP and the dashboard.

## Record

Shipped 2026-11-27 by `/loopx w4`. Triage re-checked the premise at `b18bd7abbf`: `deploy.go` still refused `scaling` on a worker ("scaling is not available on background workers", added 2026-08-02 while the operator still dropped a worker's autoscaling), and `blueprint_generate.go` still wrote an autoscaled worker as `numInstances: <max>`. Render's spec was re-read the same day: the scaling fields "apply scaling settings for the following service types: Web services, Private services, Background workers", and the pinned schema puts `scaling` on `serverService` (`web`, `worker`, `pserv`).

- **t001.** The refusal is gone. A worker's `scaling` block goes through the parse, plan, Pro gate, instance cap and disk rules a web service's does; the plan and sync were already type-agnostic, and `autoscalingEligible` already allowed workers (w4/m101). `TestBlueprintWorkerScalingAcrossSurfaces` validates the block on REST, GraphQL and MCP, syncs it and reads autoscaling on at 1..2 with CPU 60, and refuses a disk beside it with a web service's message at `services[0].scaling`.
- **t002.** Generate Blueprint writes an autoscaled worker's `scaling` (min, max, targets) and no count. `TestGenerateBlueprintDomainsCronAndWorkerScaling` now expects the block, and the export round-trip matrix (`TestGenerateBlueprintRoundTripsRealCreates`) gained an autoscaled worker on a memory target.
- **t003, dev-4.** bex-api was rebuilt from this tree (`dev-env.sh 4 up`), with a disposable verified identity and its Pro workspace (disposable). A file with one image worker `m695-walk-<hex>` (disposable), `region: frankfurt` and `scaling: {minInstances: 1, maxInstances: 3, targetCPUPercent: 60}` validated on REST and GraphQL as one `create`, and the same file with a disk was refused at `services[0].scaling`. `POST /v1/blueprints/deploy` created it and its deploy went live. `GET /v1/services/{id}` read `serviceDetails.autoscaling {enabled: true, min: 1, max: 3, cpu 60 on, memory off}`, `GET …/autoscaling` matched, and the App carried `spec.autoscaling {enabled, 1..3, CPU 60}` with the Deployment at the autoscaler's one replica. `POST /v1/blueprints/generate` wrote the same `scaling` block and no `numInstances`, and validating the export planned one `noop`. The service and both accounts were deleted (`DELETE /v1/users`, the identity lookup 404). A first try without `region` could not export: dev-env's `BEX_REGION=local-capd` has no Render alias, so any dev-N export of a service created without a region fails its own validation at `services[0].region`. That is a dev-env setting, not this change; production's `fsn1` reads `frankfurt`.
- **t004.** Render parity holds. REST, GraphQL and MCP validate share the parse and planner, and the dashboard renders their result; export is one `GenerateBlueprint` on every surface. The ADR018 autoscaling row (`docs/ADR018/services-lifecycle.md`) records the rule. Render's spec bounds the targets at 1–90, where bex takes 1–100: that drift is already filed as [w4/739](../739.md).
- **t005.** `/simplify`: the new test's manifest parser duplicated `generatedService`, so it uses that helper, and the comment on `validateManifestIngressFields` was shortened. There were no efficiency findings.
- **t006.** Each guarded line was reverted in turn and failed its tests. Restoring the refusal failed 4 tests: the parse test, the cross-surface test, and both export tests, whose self-validation refuses the exported block. Restoring the worker export exclusion failed 3: the cross-surface test, the export test and the round-trip matrix. `go test ./...` in `lego/backend` and `make lint` passed.
