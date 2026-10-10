# A combined settings update runs the pre-deploy command it clears

- **Severity:** major. A successful request to remove a pre-deploy step can still execute that step during the resulting release. The test step only printed a marker; database changes or other migration effects were not exercised.
- **Attribution:** Bex server/runtime configuration publication. Published Bex v0.3.2 and actual unmodified Render v2.27.0 / `a764810a768202704e7206eb7b87a47211fcd98e` both reproduce against Bex.
- **Context:** functional CLI QA sweep 77, 2026-10-10 UTC; `https://api.bex.co/v1/`, `bex-canary` / `tea-daif693dqjvc73e7as3g`, isolated human device OAuth, non-TTY JSON. Researched checkout `81a22ae710a39f69a717f786dd88e42067f35ac9`; deployed revision unobserved.
- **Scheduling:** worker8, seven tasks, 4h15m. Filing only; no product fix, external submission, commit or push by this hunt.

## Reproduction and controls

Create a fresh owned Free Python web service from public `https://github.com/bex-co/bex`, root `examples/hello-python`, build `pip install -r requirements.txt`, start `python main.py`, Frankfurt, auto-deploy off. Set health check `/healthz`, a unique `MESSAGE` runtime marker and a harmless pre-deploy `printf` marker. Wait for its initial deploy to become Live, verify HTTP 200 with the expected message, and capture its initial pre-deploy log ID.

Clear both settings in one native invocation:

```sh
bex services update <OWNED_SERVICE_ID> --pre-deploy-command '' --health-check-path '' --confirm -o json
```

Repeat on an independent fresh service with the actual unmodified pinned `render` binary against Bex. Both commands succeed, and API detail correctly shows an empty pre-deploy command and an omitted/empty health-check path. Both resulting deploys become Live and serve the expected HTTP marker. **Each nevertheless creates a new pre-deploy instance that prints the command's old marker after the clear request.** Final saved configuration is correct; the removed step's execution is the defect.

| Native operation | Exit / elapsed | Runtime result |
| --- | --- | --- |
| Bex combined clear on `srv-db4tbaj93q6c73at05h0` | 0 / 1.347s | Old pre-deploy marker runs in a new instance |
| Render combined clear on fresh `srv-db4tfv393q6c73at05rg` | 0 / 2.081s | Old pre-deploy marker runs in a new instance |
| Render restores both populated fields on the first service | 0 / 1.924s | Restored marker runs; populated control passes |
| Render clears only `--pre-deploy-command ''` on the first service | 0 / 1.662s | New release is Live, health remains `/healthz`, no restored marker appears after the clear |

The Bex clear began at `06:08:51.012630Z`; the new marker is timestamped `06:08:56.369887447Z`, with log ID `srv-db4tbaj93q6c73at05h0-5krfdlkephmr50m8cnj3-2026-10-10T06:08:56.369887447Z-8f0db290`. Its initial marker was at `06:07:36.655149856Z` in a different instance, `dhq5j31shkit41pjc3vu`. The resulting visible deploy `dep-db4tcsr4am4s73f098og` became Live, and detail later showed `rev-3`.

The Render clear began at `06:19:42.225233Z`; its new marker is timestamped `06:19:45.011923416Z`, with log ID `srv-db4tfv393q6c73at05rg-1vv3tfnp0gqgppasovv3-2026-10-10T06:19:45.011923416Z-4553ed4d`. Native log labels identify `type=build`, `container=predeploy`, the exact owned resource and new instance. The corresponding visible deploy `dep-db4thvr4am4s73f09950` became Live. Time-bounded native log queries and different instance IDs exclude replaying retained initial logs.

Expected: clearing the command as part of one accepted settings request publishes a release with that command removed before pre-deploy execution. The single-field clear control verifies that explicit empty strings are accepted and effective. This is not a general empty-flag failure.

## Durable evidence and request boundary

- [Native commands, exits, timings, logs, deploys, HTTP observations and API-only readbacks](evidence/live.json), plus [owned-resource cleanup ledger](evidence/cleanup.json).
- [Actual backend persisted-state observations](evidence/persisted-states.json), [probe output](evidence/source-probe.txt), [probe source](evidence/source-probe.go.txt) and [source hashes/caller census](evidence/source-metadata.json).
- [Actual pinned validator/update-builder output](evidence/pinned-body.json), [probe program](evidence/pin-builder.go.txt), [module](evidence/go.mod.txt), [checksums](evidence/go.sum.txt) and [unchanged own-service create projection used by the builder](evidence/builder-source-service.json).

Native live wire traffic was **not intercepted**. API readbacks are labeled diagnostics. The offline producer calls the actual pinned normalizer, validators and service update builder on a typed input equivalent to the observed flags; it does not execute the command parser or send network requests. It produces the correct PATCH body:

```json
{ "serviceDetails": { "healthCheckPath": "", "preDeployCommand": "" } }
```

The client preserves explicit empty values. Pinned `cmd/serviceupdate.go:114` resolves and builds the update; `pkg/types/util.go:9` preserves supplied empty strings. No launcher workaround or client fork is needed.

## Root cause

`lego/backend/internal/apps/settings.go:332` applies HealthCheckPath before `:345` applies PreDeployCommand. `ApplyServicePatch` at `:641` validates the complete patch, then `:657` enters `rollout.Batch`; `:664` still calls each present row's setter separately.

`apps/service.go:4372` SetHealthCheckPath commits its own App patch. SetPreDeployCommand at `:4021` later commits another through recordedPatch. Both reach patchTracked at `:4854` and `rollout.Tracker.Patch`. `rollout/rollout.go:133` stamps release identity on each release-changing mutation; `:162` immediately persists each Kubernetes patch before opening history. `Open` at `:238` and Batch at `:299` hold only the newest **deploy-history row**. They do not hold the App writes. The batch comment at `:280` assumes the operator only reconciles the final spec.

The private probe runs the actual current-main ApplyServicePatch with a fake Kubernetes client decorated to record the stored App after each real, non-dry-run Patch. It observes:

| Patch | Operator-visible stored states, in order | Final history rows |
| --- | --- | --- |
| Clear health + pre-deploy | Empty health with **old pre-deploy command**, then both empty | 1 |
| Clear pre-deploy only | `/healthz` with empty pre-deploy command | 1 |
| Clear pre-deploy + change start | Empty pre-deploy with old start, then empty pre-deploy with new start | 1 |

Each write carries a release stamp before the one final history row exists. The fake client does not increment metadata.generation, so the identical numeric stamps in that probe are a simulation limitation; they do not prove production generation values. This is an observation probe, not a passing test of a fix, and it executes no operator or live Kubernetes controller.

`lego/types/v1alpha1/app_identity.go:82` and `:87` classify both fields as release identity. `lego/operator/internal/controller/release_identity.go:414` consumes a newer requested release stamp. `app_controller.go:5541` skips pre-deploy only when the observed command is empty or the release is canceled; at `:5633` it passes that observed command to the Job. The first persisted state can therefore run the old command before the second state removes it. New live pre-deploy instances are observed; the exact production intermediate CR generation and Job object are source-traced, not directly inspected.

## Concrete fix and bounded consumers

Publish all CR-owned changes from a successful compound service settings request as **one complete operator-visible App configuration**, using canonical field folds and optimistic conflict handling. The existing preflight already folds requested fields into a scratch App; audit how each row owns state before selecting the staged publication mechanism. Do not blindly replace the complete App from stale scratch state or bypass source/plan store operations.

Preserve each field's validation, admission, audit and persistence semantics, one accurate history row, and the existing single-field setter behavior. Preserve save-only source updates and their deferred explicit-deploy contract. Preserve read-only empty patches, deterministic invalid-patch preflight, dry-run without mutation, and operational-only changes without phantom deploys. Unexpected store or Kubernetes failures must still return truthful errors and history for changes that actually landed; this filing does not require an invented cross-store rollback transaction.

Changing table order or deferring only the annotation is insufficient: other earlier release fields have the same intermediate-state exposure, and the operator also fingerprints direct CR release changes.

The source census records **22 table rows / 28 logical fields**, **two production ApplyServicePatch adapters** (REST PATCH at `apps/rest.go:958`, MCP update_service at `apps/mcp.go:882`) and **one production rollout.Batch caller**. Audit every row's CR/store ownership and all supported App kinds. Free web is live-proven; private service and worker pre-deploy applicability is source-only here, and their different earlier fields need explicit regression disposition. Static/cron correctly reject pre-deploy and retain their own settings contracts. Managed Postgres/Key Value do not use this App table. Shared Tracker consumers such as secrets/envgroups must preserve their existing individual operations. GraphQL and dashboard Settings currently call individual setters; no equivalent live failure is claimed there.

Regression coverage must observe intermediate persisted configurations, release identity and actual pre-deploy execution gates, alongside final saved fields/history. A test asserting only one deploy row cannot detect this defect. Replay the original combined-clear command in both real clients after deployment, with unique markers, initial/new instance IDs, current Live deploy identity, HTTP content and single-field/populated controls.

## Dedupe, lineage and limits

Searched open, blocked, done and historical board entries for combined settings, intermediate App states, Batch, command clearing and pre-deploy execution. `w6/done/m51` introduced coalesced history (targeted history identifies `910676532`); its `apps/config_rollout_test.go:302` checks one row/final release identity, not controller observation between writes. This is a runtime gap in that completed work.

`w9/m166` concerns deterministic **invalid** compound preflight; its code is already on this researched main and does not prevent this successful pair. `w5/done/m116` concerns dry-run/preflight. `w4/123` and `w4/m130` concern unexpected partial storage failures. `w4/blocked/m180` concerns no-build artifact selection; `w4/blocked/m181` concerns pre-deploy pod routing. `w4/done/187` concerns canceled-row terminal status, while `w8/done/022` concerns explicit image overrides. Environment source transactions in `w5/m44` and `w2/m73` use separate flows. None covers a successful settings request executing the command it removed. No current-main fix was found, and the actual-main backend probe still emits the intermediate state; this is not merely assumed deploy lag.

The compatibility ledger and anti-goals were checked. The pinned client/schema and actual clients against Bex provide the oracle; live Render backend behavior was not exercised. MCP, GraphQL, dashboard/browser, paid kinds, real migration side effects and direct production CR/Job inspection remain unverified. No generalized field-order failure is claimed beyond the demonstrated pair and private probe cases.

## Cleanup and filing state

Both new owned Free services were deleted by native Bex, with exit 0. Detail returned 404, workspace inventory omitted both IDs, public runtimes returned 404, all six immutable baseline services remained, and final quota was six services with zero terminating. Four deploy children on the first service and two on the mirror were enumerated before parent deletion; physical Kubernetes descendant teardown was not directly inspected. Final readonly quota checks converged at approximately `06:23:24Z` and `06:24:11Z`. The copied Render config was removed without logging out the original isolated QA grant.

All evidence is durable and sanitized; temporary auth remains private for the continuing hunt. This milestone is scheduled and unimplemented. This filing is uncommitted at creation.
