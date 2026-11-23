# w4 · m596 — Render's OpenAPI of 2026-11-23: sandbox suspend, a build's deploy scope, and logs and metrics by project

**Worker:** worker4 **Goal:** bex answers Render's live public API of 2026-11-23 on every operation it serves. `POST /v1/sandboxes/{id}/suspend` suspends a sandbox. A build-source build takes Render's `deploy.serviceScope` and answers the deploys it started. The three logs reads and the 18 metrics reads take Render's `project`, `environment` and `region`. The pin and ADR018 record the round, so the weekly `render-schema` drift job is green again. **Status:** blocked — pin must carry `3e9c94d635` before t009 runs the live production checks in `bex-canary`

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Re-pin Render's 2026-11-23 OpenAPI and land the dispositions that need no new behavior | 1h | — |
| [t002](done/t002.md) | A build-source build takes Render's `deploy.serviceScope` and `serviceIds`, and answers the deploys it started and the services it skipped | 1h45m | t001 |
| [t003](done/t003.md) | `POST /v1/sandboxes/{id}/suspend` suspends a sandbox with Render's 202 and `sandbox_not_running` | 50m | t001 |
| [t004](done/t004.md) | The three logs reads take Render's `project`, `environment` and `region` | 1h30m | t001 |
| [t005](done/t005.md) | The 18 metrics reads take Render's `project`, `environment` and `region` | 2h | t001 |
| [t006](done/t006.md) | Render parity, the round 9 record in ADR018, and a green drift dispatch | 45m | t002, t003, t004, t005 |
| [t007](done/t007.md) | Simplify | 15m | t006 |
| [t008](done/t008.md) | Test coverage | 45m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

Total: **9 tasks, about 9h**.

**Ship order.** t001 and t002 ship in one push. The re-pinned 201 of `trigger-build-source-build` requires `deploys` and `skippedServices`, which bex does not answer before t002. `rendertest`'s `Allowlist` only shrinks (`TestAllowlistOnlyShrinks`), so the gap cannot be allowlisted in between. t003, t004 and t005 may ship one at a time after that.

## Definition of done

- `bash scripts/render-schema-drift.sh` reports no REST operation drift and no operation schema drift against Render's live document on the day it runs. If Render has moved again since 2026-11-23, the new delta is dispositioned the same way.
- A `render-schema-drift.yml` dispatch on `main` is green on `render-schema`, or red only on the CLI pin with w22/026 named as its owner.
- ADR018 records round 9: every changed parameter, body field and response field is served, refused by name, or documented with an owner.
- On production, once the pin carries the change, signed in as the QA user (`bash scripts/qa-login.sh`) in `bex-canary`, using a disposable project, build source, services and sandbox that the run deletes afterwards:
  - `POST /v1/sandboxes/{id}/suspend` on a running sandbox answers 202, the sandbox then reads `suspended`, and a second call answers 202. A terminated sandbox answers 409 `sandbox_not_running`.
  - `POST /v1/build-sources/{id}/builds` with `{"deploy":{"serviceScope":"none"}}` answers 201 with `"deploys": []` and starts no deploy. With `custom` and one linked service, it answers that service's deploy, which ships the build. With no `deploy` it deploys every linked service that has autodeploy on, as before.
  - `GET /v1/logs?ownerId=…&project=<prj>` and `GET /v1/metrics/cpu?project=<prj>` answer for the project's resources without any `resource`. A project the caller cannot see answers 404 on metrics.

## Source + Goal linkage

- **Source:** inbox note w4/722 ([`source-722.md`](source-722.md)), filed by the `/loopx w4` orchestrator on 2026-11-22 from drift run [45095344162](https://github.com/bex-co/bex/actions/runs/45095344162). It was promoted by `/loopx w4` on 2026-11-23, after `scripts/render-schema-drift.sh` was re-run against Render's live document. The work is four behavior changes across four packages (deploys, sandbox, logs, metrics) plus the re-pin, about 7h before the closing tasks, which is well past the inbox's hour.
- **Premise checked 2026-11-23 on `2a7fd6fc56`.** Render's live `render-public-api-1.json` has sha256 `d075b0cb…` and 265 operations (166 paths, 170 schemas, 74 request bodies). The pin is `77508de0…` with 264 operations, from round 8 (`9c58dd1acd`, w8/m442). No commit since then has re-pinned. `11f5f7acdb` (joins repeated explode=false keys), `ffc0499035` (multi-value `statusCode`, w34/010) and `d67b22b17b` (Render's `errorCode` vocabulary, w5/m583) changed the gate and the codes, not the pin. Since the note was filed, Render has also changed `GET`/`POST /build-sources/{buildSourceId}/builds` and `GET …/builds/{buildId}`. The drift now lists 33 changed operations, not 31.
- **Today's diff and its owner, pinned `77508de0…` → live `d075b0cb…`:**
  - **Added `suspend-sandbox`** (`POST /sandboxes/{sandboxId}/suspend`). It answers 202 with no body once the sandbox starts suspending, or when it is already `suspending` or `suspended`. Otherwise it answers 409 `sandbox_not_running`. bex suspends through `sandbox.Service.Suspend` (`lego/backend/internal/sandbox/service.go:1878`), mounted only as the bex extension `POST /v1/sandboxes/{id}/pause`, which answers 204 (`sandbox/rest.go:226-229`) → **t003**.
  - **Build-source builds.** `trigger-build-source-build` takes `deploy: {serviceScope: autodeploy_only | none | custom, serviceIds}`. Its 201 adds the required `deploys: [{serviceId, deployId}]` and `skippedServices: [{serviceId, reason: suspended | deploys_shut_off}]`. The list and retrieve reads only moved the build schema; the shape is unchanged. bex builds first and fans out to linked services after the build succeeds (`deploys/build_sources.go:116-131`, `:320-380`), so no deploy id exists when the trigger answers → **t002**.
  - **Logs** (`list-logs`, `subscribe-logs`, `list-logs-values`): `resource` is required only when `project` is absent. They add `project` (the project's services, cron jobs, Postgres and Key Value), `environment` (requires `project`) and `region` (requires `project`; required when the resources span regions). The 400 documents `multiple_regions` and `too_many_resources`. Today bex's list and values refuse `project` ("project is taken only by /v1/logs/subscribe", `logs/rest.go:810-813`). The tail takes `project` as a bex extension with `filterDocument` (w3/m505, `render_openapi.go:282-285`). GraphQL takes exactly one of `resource` and `project` (`logs/graphql.go:243-256`), where Render's `resource` narrows a project → **t004**.
  - **Metrics** (18 `GET /metrics/*`): they add `project`, `environment` (if both are given, the environment must belong to the project) and `region`. `resource` becomes optional when either is given, and each named resource must belong to it. A project or environment the caller cannot see answers 404. The new `400MetricsBadRequest` documents `too_many_resources` (more than 50) and `multiple_regions`, and `bandwidth-sources` keeps its cap of 30. `parseMetricParams` answers "resource is required" without one (`metrics/rest.go:425-431`) → **t005**.
  - **No behavior change, recorded in t001:**
    - `add-resources-to-environment` (`POST /environments/{id}/resources`) now answers 201 Created, not 200. bex answers 200 (`environments/rest.go:226`).
    - `create-sandbox-group` now pins `environmentId`, which bex accepts as a body extension (`render_openapi.go:331`, w22/m412).
    - `postgresDetail.project`, `notificationOverride.type` and a metric value's `unit` are no longer required; bex still sends them. `rendertest`'s `dropUndefinedMetricValueUnit` may now be dead.
    - The sandbox create and list responses were restructured into an `allOf` of the same sandbox.
    - `workflowversions` dropped two required fields; it is Workflows, an omission.
  - **CLI.** The same job reports render-oss/cli v2.38.0 past the v2.37.0 pin. w22/m444 shipped v2.37.0 on 2026-11-21, and ADR018's round 8 names **w22/026** as the owner of v2.38.0, not this milestone.
- **Render:** the operation and parameter descriptions in the live document above are the reference. t006 also reads Render's changelog and docs for each of the four features.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)). A client generated from Render's current spec, such as a future CLI's regenerated REST client or Render's Terraform provider, sends these parameters and bodies and parses these responses.
- **Expected outcome:** a Render client can suspend a sandbox, choose which services a build deploys, and read logs and metrics for a whole project or environment on bex as on Render. The weekly drift job is green, so Render's next change shows as the only red.
- **Why now:** `render-schema` has been red since the 2026-11-16 schedule ([43902485998](https://github.com/bex-co/bex/actions/runs/43902485998)), and a red that is already expected hides every later change. The diff has grown since the note was filed (from 31 to 33 changed operations), and each round only gets larger.
- **Render parity:** included (t006). Every task changes a REST operation that GraphQL and MCP mirror, and the dashboard's project logs view already reads by project.

## Record

- **2026-11-23 — t001–t008 shipped in `3e9c94d635`:** OpenAPI re-pinned to `d075b0cb…` (265 ops / 166 paths / 170 schemas / 74 bodies), `POST /v1/sandboxes/{id}/suspend` (202, retiring `/pause`), build-source `deploy.serviceScope`/`deploys`/`skippedServices` (migration `0341`), shared `savedsearch.ResolveScope` + project/environment/region scope and `too_many_resources` on logs and all 18 metrics, `201` on `add-resources-to-environment`, ADR018 round 9 + `docs/render-artifacts/openapi-rebaseline-round9.md`, and `allowlistSize` lowered 22 → 18.
- **Drift dispatch (`t006`):** [45239352057](https://github.com/bex-co/bex/actions/runs/45239352057) on `main` (`3e9c94d635`): `render-mcp`, `render-changelog`, and `render-examples` green; `render-schema` passed Blueprint schema, 68-value webhook enum, 265 REST operations, and all 220 served REST operation schemas, and failed only on `render-oss/cli v2.38.0` vs pin `v2.37.0` (owned by `w22/026`).
- **Blocked on deploy pin (`t009`):** production's image pin (`9fdf7e0a1171` in `cded300699`) does not yet carry `3e9c94d635`. Once the pin carries `3e9c94d635`, run the three live production checks above as the QA user (`bash scripts/qa-login.sh`) in `bex-canary`, delete the disposable fixtures, and close with `t009`.
