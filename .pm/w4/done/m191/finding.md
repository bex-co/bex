# Health-mode edits roll an instance under the old release revision

**Severity: major.** Healthy hosting continues, but the deployment history and served revision misrepresent a real configuration rollout.

## Reproduction and actual outcome

Workspace bex (`tea-d98210cbbpdc73dcrkvg`), English desktop dashboard. Two owned Free web services, public image `docker.io/library/busybox:1.37`, port3000, one instance, no project/environment, custom domain, environment value, secret file, disk or pre-deploy step.

Use Existing Image with this Docker Command; replace the marker suffix for a second fixture:

```sh
mkdir -p /tmp/qa-www; echo QA_A33R_HTTP_OK > /tmp/qa-www/index.html; echo QA_A33R_HEALTH_OK > /tmp/qa-www/healthz; echo QA_A33R_START; exec httpd -f -p "$PORT" -h /tmp/qa-www
```

1. Wait for the default empty/TCP health deployment to become Live. Edit Health Check Path to `/` and save. Reload the deploy detail; read `service` and `deploys`; request the public root.
2. The first fixture `qa-20261009-loop-a33-health` / `srv-db4ejbr93q6c73at02pg` initially served rev-1. The root-setting row `dep-db4ek0b4am4s73f0969g` was created at2026-10-09T13:20:01.594376Z, never acquired startedAt, and finished **canceled** at13:38:13.578021Z, **18m11.984s**, with “Superseded by a newer release.” The complete history at the terminal observation contains only this row and the original create. Service remained Running rev-1, saved path `/`, undeployedChanges:false; the public root returned200.
3. Fresh independent fixture `qa-20261010-loop-a33-reverse` / `srv-db4td5j93q6c73at05jg`: default TCP became Live rev-1; `/healthz` became Live rev-2 in12.687s; `/` became Live rev-3 in18.894s. Clearing `/` at06:11:31.802609Z created `dep-db4te4r4am4s73f098s0` but left Running rev-3. At06:12:28Z, REST and MCP still said created with unchanged updatedAt; GraphQL agreed, and public HTTP remained200. A different instance logged START at06:11:35.794175355Z, proving an instance replacement despite the unchanged release revision.
4. On the first fixture, the positive recovery `/` → `/healthz` became Live rev-3 at06:13:27.626724Z. Clearing `/healthz` became Live rev-4 at06:14:27.660192Z. Fresh-load Settings, set `/` again at06:14:58.544332Z: row `dep-db4tfoj4am4s73f0990g` stayed created on a new page load at06:16:18Z, with Running rev-4 and empty timing/diagnosis fields. A new instance logged START at06:15:00.432243923Z and the public root returned200.
5. Both short repeats were intentionally canceled via Cancel + Proceed before cleanup. Only the first row was observed through its natural18-minute terminal outcome; do not describe the repeats as naturally timed-out or unbounded.

**Expected:** TCP and HTTP root are distinct health configurations. Each actual transition opens one configuration deployment, advances the operator's release generation/revision while retaining the artifact, and becomes Live once that release is healthy. Existing requests keep reaching the healthy fixture. A genuine no-op creates no deployment.

## Producer, representation and consumer

- `lego/backend/internal/apps/service.go:4372` preserves the normalized empty or rooted path through patchFetched. `settings_check.go:106` trims whitespace but does not turn empty into `/`. `types/v1alpha1/app_types.go:623` declares a string with no CRD default, so both values are expressible.
- `types/v1alpha1/app_identity.go:82,126` classifies HealthCheckPath as release input and compares its raw value. `backend/internal/rollout/rollout.go:133–145` stamps the post-patch release generation and opens its tracked row through Patch. That correctly treats empty ↔ `/` as a real release.
- **Root cause:** `operator/internal/controller/release_identity.go:152–156` converts empty to `/` before putting HealthCheckPath in releaseIdentityInput at173. Thus two distinct probe modes have the same fingerprint input. This is application normalization, not a hash collision.
- `release_identity.go:289–298` updates ReleaseGeneration only when that release fingerprint changes. The collision leaves the operator's release generation behind the newly stamped backend row.
- `deployment_projection.go:298–306` correctly projects empty as TCPSocket and `/` as HTTPGet. Its single caller at263 fans out to startup/readiness/liveness. `release_config_snapshot.go:648` / `app_controller.go:2461` apply the changed template, explaining the observed replacement instance even though identity did not advance.
- The history consumer `backend/internal/store/reconciler.go:796` requires the observed release generation to match the open row; `releaseIsActive:1556` also requires its rev-N. `supersededDeployStatus:1454–1486` holds a future row until the gate expires, then cancels it. `deployTimedOut:1283–1300` uses the Created row's18-minute image-deploy gate. The first capture's exact terminal time fits this path. Do not “fix” it by declaring a mismatched generation Live or weakening that backstop.
- `release_config_snapshot.go:440–463` records the last template under the current generation, so a same-generation probe replacement can overwrite the old record. This is a code-derived consequence, **not** a production record-read observation.

**Framework checked:** operator go.mod pins Go1.26.0. Opened [that version's encoding/json encoder](https://raw.githubusercontent.com/golang/go/go1.26.0/src/encoding/json/encode.go): isEmptyValue336–348 tests string length; structEncoder698–710 omits empty fields and encodes nonempty ones. releaseIdentityInput's string/omitempty field at90 can distinguish absent/TCP from explicit root without an API or CRD type change. The information is lost before serialization by the normalizer. The deployed Go patch version was not queried.

## Fix specification and shared scope

Preserve health-mode distinction in release and settings identities. An actual switch must adopt the generation opened by the backend and retain the previous release record; changing probes must not change artifact identity or rebuild source solely for that reason. Keep identical-path no-ops and operational metadata/spec changes independent of release identity.

**Migration is part of the fix.** Removing the fallback alone changes the fingerprint of every previously stored TCP App. Define compatibility for both status.releaseFingerprint and recorded SettingsFingerprint. Before normal release classification, derive or validate the previously applied mode from authoritative retained/applied probe evidence and the legacy identity inputs. Rebase an unchanged legacy identity without changing pods, release generation or history; distinguish a real pending edit and let it complete under its owed generation. Missing/ambiguous evidence must not be treated as equality or manufacture a served template from current mutable spec. Preserve existing canonical missing-status handling, selected release inputs and the build/source-pending gates. Choose and document a version/adoption strategy in t002; do not bump a global prefix and roll the fleet.

Production caller census (`rg`, test files excluded):

| helper | production calls | consumers |
| --- | --- | --- |
| desiredAppReleaseIdentity | 2 | prepareAppReleaseDecision:257; releaseSettingsFingerprint:192 |
| prepareAppReleaseDecision | 1 | app_controller.go:595 |
| releaseSettingsFingerprint | 2 | saved_configuration.go:175 comparison; release_config_selection.go:324 record write |
| healthCheckHandler | 1 | deployment_projection.go:263, shared by three probes |
| appContainer | 1 | deployment_projection.go:417 |
| applyServingDeployment | 1 | app_controller.go:2461 |

The identity helper applies globally to App releases; the fix must retain existing non-health inputs and operational exclusions, with explicit tests for consumers already correct. saved_configuration.go:138–175 uses the record's settings fingerprint for cancel equality; release_config_selection.go:95–100 derives a legacy path from the actual readiness probe. ReleaseRecordSpec (`types/v1alpha1/releasesnapshots.go:93–116`) uses a path pointer to distinguish empty from legacy missing and stores a settings fingerprint string; preserve that distinction.

**Aliases:** dashboard Settings uses GraphQL setHealthCheckPath (`apps/graphql.go:1662`); REST PATCH `/v1/services/{id}` accepts top-level and serviceDetails.healthCheckPath (`rest.go:834–846`); MCP update_service (`mcp.go:812,882`) shares ApplyServicePatch's health row (`settings.go:336–341`). Creation and Blueprint/direct CR input can carry the same field. These write aliases are traced; only the dashboard setter was mutated live. REST/MCP deploy readers were probed. Do not list the retired set_health_check_path MCP tool as current.

**Seven resource families:** web (including legacy empty type) and private services reach the networked app-container probe projection; only web was exercised. Static sites branch to the shared static-server at app_controller.go:2405; cron branches to CronJob at2400; worker has worker:true and no probes. Those App types still traverse the shared identity classification and need preserved controls. Postgres and Key Value use separate Database/KeyValue controllers, not this App identity helper. Do not infer seven live failures.

**Adjacent functional states:** preserve valid non-root HTTP changes and default TCP; rejected non-rooted input; padded/identical path no-ops; omitted PATCH versus explicit clearing; normal in-progress deployment; genuine failure, user cancellation and newer-release supersession; operational scaling/rename behavior. No new endpoint, health header or lifecycle status is needed.

**Tests currently miss the edge:** release_identity_test.go:422's cross-plane agreement test mutates strings through mutateIdentityTestField:379 by appending “changed”. It exercises empty → changed, not empty ↔ `/`. Add explicit paired cases plus release-generation/history and retained-record behavior, rather than another field-presence assertion.

## Dedupe and precedent

Searched all .pm markdown, including done/blocked; scanned53 non-done milestone READMEs across all workstreams plus workstream queues; reread DO_NOT_DO, latest40 dashboard/lego commits and targeted git log -S. HEAD was `c5e413f38324fdff0a36f4c3edc0063e889ce639`. The normalizer is still present; -S points to `b58265e6c` (w2/m56), not an undeployed fix. Before shipping, pulled through `bb8abb901` and checked the new w8 notes and m55 (54 non-done milestones before this filing): no overlap, and the operator/backend root-cause code is unchanged.

- **w4/m190:** open HTTP Host-header issue. Its fixture fails readiness because the application rejects the Pod-IP Host. Here both health routes serve200 and non-root checks become Live; the defect is identity/history, not the request Host. No overlapping identity/adoption task exists there.
- **w7/m80:** TCP-default contract introduced after w2/m56's root canonicalization. This is the remaining identity edge; the setter's old empty-to-root coercion is already gone. Walk of its whole DoD: default TCP was Live in this pass; 2.5s response and8-minute boot were not exercised, so retain their existing tests; timer relationship was read but not retimed; explicit HTTP `/healthz` and `/` became Live. Do not reopen its default, timeout or probe-constructor fixes.
- **w2/m56:** preserve its entire boundary: source-backed scale without build/pre-deploy/pod replacement; real deploy with one build/pre-deploy and new revision; legacy adoption; honest asynchronous dashboard scale acknowledgment. These are compatibility cases not live-tested here. The defect is not a claim that operational scale now deploys.
- **w6/m51:** its six DoD bullets cover Start Command row creation, UI/Events history, failing-command terminal state, correction-only recovery, the full Settings family and the16-site App-writer census. Start Command/Events/failing-build/env-group journeys were not repeated here; preserve those tests and recheck affected call-site classification in t003/t006. This pass confirms health changes already create rows, then exposes a value-pair the cross-plane agreement test did not cover. Treat it as a residual pairwise identity gap, not a return of the old absent-row bug.
- **w4/m165:** preserve identical-cancel false versus real retained-change pending; changing settings fingerprint requires compatibility, not a duplicate of its cancel-banner task.
- **w4/211:** its non-root HTTP → TCP control remains Live here; it owns the separate activator diagnostic.
- **w9/m166:** atomic patch validation does not repair the operator identity. w9/m89/m92 precedent has no health-mode history finding. No anti-goal applies.

## Evidence, limits and cleanup

[evidence.md](evidence.md) is the durable exact-request/complete-response record. Verified local artifacts: `.playwright-mcp/qa-health-mode-a33-ledger.json`, `qa-health-mode-a33-canceled.png`, `qa-health-mode-a33-reverse-created.png`, `qa-health-mode-a33-console.log`, `qa-health-mode-a33-network.log`. Both screenshots were opened and inspected. The first shows the false terminal reason and rev-1; the second shows Created versus Running rev-3. Console capture had0 errors. Network401s belonged to an expired QA session and navigation aborts came from route changes; neither was filed as a product bug. Valid observed mutations/readers returned200.

Both repeats were deliberately canceled and both owned services deleted through the UI. At06:17:34Z, both service APIs and public hosts returned404; services5/databases4/keyValues2/projects3/envGroups1/blueprints1 matched the pre-existing inventories. Other workers' QA resources remained untouched. Logout succeeded, the old browser cookie returned401, all browser cookies were cleared and the run's jar/state handle removed.

**Unverified live:** raw CR release generation/probe objects/record overwrite, legacy upgrade or missing records, source rebuild avoidance, rollback/cancel-template restoration, direct CR edits, concurrent builds, Blueprint updates, private/static/worker/cron siblings, and Events/notification delivery. Cover the affected behavior in tasks; these are not extra live findings. The repeat rows' natural18-minute terminal outcomes were not observed.

**Render:** [health-check documentation](https://render.com/docs/health-checks), checked2026-10-10, documents default TCP and opt-in web HTTP paths with2xx/3xx success. Exact path-edit deployment/revision timing was not tested on Render. The requirement for an owed bex configuration release to finish Live comes from ADR004 and the shared backend/operator tracking contract, not an invented Render wire promise.
