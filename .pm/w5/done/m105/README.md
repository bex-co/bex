# w5 · m105 — Settings command edits take effect in the deployed artifact

**Worker:** worker5 **Goal:** A Settings command edit deploys the requested behavior instead of silently retaining an old baked command. **Status:** blocked (t001, t002, t005, t007 done; original production reproduction gate remains)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Research Render command-edit deploy behavior for parity — **DONE** | 30m | — |
| t002 | Reproduce native Settings command edits on dev-5 — **DONE** | 50m | t001 |
| t003 | Repair the demonstrated artifact dispatch or selection failure | 55m | t002 |
| t004 | Verify command effects and deployment controls | 45m | t003 |
| t005 | Record cross-surface runtime evidence — **DONE** | 30m | t004 |
| t006 | Render parity | 30m | t001, t002, t003, t004, t005 |
| t007 | Simplify — **DONE** | 20m | t006 |
| t008 | Test coverage | 45m | t006, t007 |
| t009 | Closeout | 15m | t007, t008 |

## Definition of done

- [x] Dated command-edit/deploy/cache/restart/rollback matrix is recorded.
- [x] Acceptance tests observable command effects rather than imposing an unverified Render image mechanism.
- [ ] Native start/build edit and unchanged-spec controls have timestamped artifact/runtime evidence.
- [ ] If local behavior passes, production uncertainty remains open; no speculative correction is made.
- [ ] The demonstrated failure has a named cause and a targeted correction.
- [ ] No change bypasses artifact identity or corrupts rollback/reuse behavior.
- [ ] Saved start and build commands take effect in the intended deployed revision.
- [ ] Control cases retain their documented reuse/rebuild behavior.
- [ ] Failed builds never masquerade as a successful updated artifact.
- [ ] Evidence proves the user-visible command effects without treating every digest change as mandatory.
- [ ] No production reproduction or resolution is claimed from local evidence alone.

## Source + Goal linkage

- **Source:** [w4/m110/t001](../../../w4/done/m110/t001.md); approved by the user on 2026-09-30 following the w5 brainstorm and parity correction.
- **Goal linkage:** ADR008 dependable, deterministic hosting and agent operation.
- **Expected outcome:** A Settings command edit deploys the requested behavior instead of silently retaining an old baked command.
- **Why now:** The central edit-to-deploy contract has an unresolved observed failure; static identity tests already pass and runtime diagnosis is the next useful step.
- **Render parity:** Included because tenant-facing behavior changes. t001 researches Render first; the closing parity task verifies the finished behavior. Label unmatched surfaces as bex extensions, not proven Render parity.
- **Sizing:** 210m research/implementation; 320m including closing tasks; 9 tasks.

## Transfer and execution constraints

Only w4/m110/t001 implementation ownership transfers. Its completed instance-count, rollback-label and log-narration fixes remain complete and out of scope. w4/m110 retains the original production acceptance gate until evidence actually satisfies it. Use dev-5; local provisioning/recovery is pre-approved within harness boundaries. A passing local reproduction is not proof the production incident is resolved: document the remaining measurement needed and keep the item open. Do not invent an implementation merely to finish the queue.

## Render research — 2026-09-30 (t001)

Documentation evidence only; no authenticated Render command-edit capture is available in this run.

| Operation | Documented Render behavior | Acceptance here |
| --- | --- | --- |
| Save build/start command | Configuration changes deploy regardless of build filters ([monorepo FAQ](https://render.com/docs/monorepo-support#faq)). | Saved command must take effect in the resulting successful revision. |
| Build/start execution | Deploys run configured commands; a failed step fails the deploy and preserves the last successful service ([deploys](https://render.com/docs/deploys#deploy-steps)). | Check build and runtime markers, plus failure preservation. |
| Clear cache and deploy | Clears prior build cache; ordinary manual deploy can use cached work ([manual deploys](https://render.com/docs/deploys#manual-deploys)). | Distinguish cache reuse from stale command execution. A new digest is diagnostic evidence, not a Render promise. |
| Restart | Uses the running revision's commit/configuration, including prior environment values ([restart](https://render.com/docs/deploys#restarting-a-service)). | Do not treat restart as deployment of saved settings. |
| Rollback | Reuses target build artifact, start command and service environment; does not overwrite saved current configuration ([rollback configuration](https://render.com/docs/rollbacks#whats-rolled-back)). | Verify target runtime and subsequent normal deployment separately. |

Bex's artifact fingerprint, generation tags and native baked start command are implementation choices. GraphQL/MCP are bex surfaces, not demonstrated Render equivalents. The original source's digest-based acceptance is interpreted as evidence for this implementation; observable commands are the tenant contract.

## Local investigation — 2026-10-01 (in progress)

Source HEAD `6c32dfd42`; isolated dev-5 API/dashboard, public Node fixture `srv-dav09d9jg4r3nn5e1410` in `tea-dav0901jg4r3nn5e13og`, repository `render-examples/express-hello-world`, resolved commit `039c34770852fb07cef7f9f0f8534c5de408b207`.

- Existing `TestSettingsCommandEditsDemandAFreshArtifact` and release/no-op/cancel/cache identity controls pass. Backend `internal/apps` and `internal/rollout` suites pass. These are automated/static evidence, not production acceptance.
- Local cluster has no bex manager. A temporary Go test overlay calls the real `AppReconciler.Reconcile` periodically for this single App; writes are limited to dev-5, its disposable build namespace and this workspace. This proves behavior **after reconciliation is invoked**, not manager event/watch delivery. No tracked source is changed.
- BuildKit requires the dedicated privileged build namespace `dev-5-build`; the first Job under dev-5 was rejected before Pod creation. Tenant namespace policy was not weakened.
- Canonical registry addressing uses a dev-5 Zot NodePort plus existing node registry trust; only fixture Job host aliases and host-side registry transport are adjusted. Public ingress is unconfigured, so runtime proof uses Pod readiness/logs.
- Shipped Skopeo digest `64ac45c5a1c01230896fbae960b2213e32a5040e4009b83b5f5cbf31a35f61c3` returns authenticated registry `404 MANIFEST_UNKNOWN`. Same-version OCI index `2b7d76dde38c5924e1945348840c58786fc192201f2fd545832e58b53d26f7b8` is addressable; fixture-only arm64 digest `ff8e427206bcae14f99ab43305b6abed494572ba1cdd526cea6511353a5cd91a` permits push. This is an explicit test accommodation, not a shipped correction. Existing owner **w1/blocked/108.md** already tracks this pin; do not duplicate it or claim it caused the older incident.
- Correction to historical source acceptance: explicit native manual/hook deploys stamp `spec.restartedAt` (`deploys/service.go`), which enters artifact identity (`release_identity.go`). They request a build; cache may reuse work. A no-op Settings save, an unchanged reconciliation, and rollback are separate controls. The old source's unconditional manual-deploy reuse expectation is not the current contract.

| UTC action/observation | Local result |
| --- | --- |
| 06:59:02 create, gen1 | Build prints `W5_BUILD_INITIAL`; runtime prints `W5_START_INITIAL`; Ready 07:07:23. |
| 07:10:23.988 dashboard start-command confirm, gen2 | GraphQL returns saved `echo W5_START_EDIT && npm start`; new Job at 07:10:25; runtime marker 07:11:03; Ready 07:11:11. Prior pod stays available during build. |
| 07:11:28.337 dashboard build-command confirm, gen3 | GraphQL returns `npm install && echo W5_BUILD_EDIT`; new Job builds and reaches Running/rev3. |
| 07:12:46.696 same build command via GraphQL | No-op: metadata generation remains 3, deploy count remains 3. |
| 07:12:56.798 explicit manual REST deploy | 201 `dep-dav0fu1jg4r5ujjdip0g`; gen4 begins a fresh build with commands retained. |

REST and MCP `get_service` expose the saved command consistently with GraphQL and Settings. The MCP read was real authenticated HTTP/SSE; it does not establish Render's equivalent mutation contract. Detailed build/Pod/status captures are temporary under `/tmp/w5-m105-evidence`; the final durable result will be summarized here after controls and cleanup.

### Additional controls (UTC, 2026-10-01)

- Gen3: `W5_BUILD_EDIT` at 07:12:07 and runtime `W5_START_EDIT` at 07:12:19; image digest `83b53598029d4c0ee23713d445a822d001d64ac7e0c94b851d38ed00ec900901`.
- Gen4 manual deploy: 39-second successful Job, Ready; command markers retained.
- Gen5 non-secret CR literal `W5_CONTROL=env-ok`: 42-second successful Job; `printenv W5_CONTROL` returned `env-ok`. This is an operator-level control, not an API write.
- Gen6 MCP `update_service` at 07:16:40 set build command `echo W5_BUILD_FAIL && exit 23`: Job terminal Failed at 07:17:17, API `build_failed` at 07:17:21; App stays Running with active `rev-5` and unchanged gen5 image. Failure is determined from the Job, not a timer.
- Gen7 REST PATCH at 07:18:01 restored the prior valid command: Ready at 07:18:14; no new build Job, because the last successful artifact has exactly those inputs.
- API environment write initially returned 503 because base dev-5 has no secret store. Added a disposable OpenBao in dev-5-auth, client SA with no Kubernetes RBAC, server SA with TokenReview permission only, KV policy scoped only to this fixture workspace. No shared agent stack or production credentials. A conflicting write to the CR-owned literal correctly returned 409; separate API-owned `W5_API_CONTROL=api-env-ok` returned 200 at 07:20:50. Gen8 rebuilt in 38 seconds, Running at 07:21:44; actual pod `printenv W5_API_CONTROL` returned `api-env-ok`.
- The session was interrupted by model capacity before rollback. The 40-minute temporary reconciler expired by design (test harness timeout, not a product regression); the fixture remained healthy. Resumed at 21:00 UTC, renewed only its dev credential and restarted the same scoped harness. Restart with no request body returned 200 at 21:00:18. Earlier `{}` was correctly refused by the no-body endpoint (400); expired browser session correctly returned 401 before reauthentication.

### Resumed controls and disposition — 2026-10-01

Restart gen9 became ready with `W5_START_EDIT` and `W5_API_CONTROL=api-env-ok`. Rollback REST returned 201 at 21:01:55, selected original gen1 image, created no gen10 build Job, and its ready gen10 pod printed `W5_START_INITIAL`. The next normal deploy returned 201 at 21:02:26; gen11 became ready and API history reached `live`. The restart/rollback history rows were superseded to `canceled` because the next control ran after Kubernetes readiness but before the API's periodic status observation. This run proves their ready Pod behavior, **not** terminal-live API history for those two intermediate rows.

**Independent rollback parity finding (recorded, not silently called a pass):** REST immediately after rollback serves `startCommand=echo W5_START_INITIAL && npm start`, and the API-owned `W5_API_CONTROL` is gone from saved environment. Gen11 also prints `W5_START_INITIAL`. `deploys/service.go` `RollbackWithOptions` explicitly restores environment into saved state and overwrites `spec.startCommand`. That differs from Render's documented preservation of current saved configuration. Disposition: retained here as an unscheduled rollback-settings follow-up, outside the transferred stale-command build-dispatch correction (and distinct from the completed rollback-label work). No correction or exact rollback parity is claimed.

**Verdict: BLOCKED.** Local start/build edits successfully execute the requested commands, so the original stale-artifact failure is not reproduced. The missing gate is user authorization for the narrow production reproduction described by w4/m110/t001 (one disposable native service, command edit, App/operator/build evidence, then delete), or equivalent production incident evidence supplied by an authorized operator. The single-App harness cannot establish manager-watch delivery or production resolution. Do not manufacture an artifact-identity/container-command patch. t001 research, t002 local reproduction and t005 evidence are complete; t007 has no source diff to simplify. Correction-specific regression/acceptance and closeout remain blocked.

Durable sanitized snapshots: [native controls](evidence/native-controls.json). Raw local logs remain under `/tmp/w5-m105-evidence`; no private credentials are committed. Existing operator identity controls and backend apps/rollout suites pass; no product source changed, so no new regression test can honestly be claimed to fail before a nonexistent correction.

### Cleanup

Deleted service through REST (204), then removed only its known finalizer after deleting the disposable registry (host-side registry GC cannot resolve cluster DNS in this harness). Deleted dedicated build namespace, Zot, OpenBao, its two service accounts and named TokenReview binding, forwards and private credentials. Identity deletion returned 204. Workspace API cleanup raced the API configuration restart and failed while it still pointed at the removed isolated Bao; removed exactly the fixture tenant row from dev-5's database and its now-empty namespace. Standard dev-5 API configuration restored; no other workstream resources changed.
