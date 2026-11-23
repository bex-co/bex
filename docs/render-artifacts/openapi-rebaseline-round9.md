# OpenAPI drift re-baseline round 9 (2026-11-23, w4/m596)

Re-baseline of Render's live public API (`https://api-docs.render.com/openapi/render-public-api-1.json`) against round 8 (`77508de0…`, 264 operations, `w8/m442`). The Blueprint schema (`6c6721a6…`), webhook enum (68 values), and MCP tools pin (`main@a18ef3ee`) are unchanged.

## Pin summary

- **OpenAPI digest:** `d075b0cbc6fece3751223af91a497bd6a689ec95385e9117996bbda6928072ce` (485,803 bytes)
- **Guard counts:** 166 paths, 170 schemas, 265 operations (+1 added, 0 removed), 74 request bodies (+1 extension)
- **CLI pin:** `render-oss/cli` v2.38.0 (2026-11-20 UTC) is newer than bex's v2.37.0 pin; owned by `w22/026`.

## Operation and schema dispositions

### 1. Added operation: `POST /v1/sandboxes/{sandboxId}/suspend` (`suspend-sandbox`)

- **REST:** Answers `202 Accepted` with no body on a `running` sandbox, idempotent `202` on `suspending` or `suspended`, `409 Conflict` with `"code": "sandbox_not_running"` on any other state (including `terminated`, `creating`, `resuming`, `errored`), and `404` for unknown or cross-workspace sandboxes.
- **Retired alias:** Retired bex's pre-spec `POST /v1/sandboxes/{id}/pause` (`204`) in favor of Render's `/suspend` (recorded in [ADR104 D2](../ADR104-sandbox-render-parity.md#d2--lifecycle-timeouts-and-suspension)).
- **GraphQL & MCP:** Added GraphQL `suspendSandbox(id: String!, ownerId: String): Boolean!` and MCP `suspend_sandbox` (bex Extension; total MCP tools 288, Extension 260).

### 2. Build source deploy scope: `POST /v1/build-sources/{buildSourceId}/builds` (`trigger-build-source-build`)

- **Request:** Accepts optional `deploy: {serviceScope: "autodeploy_only" | "none" | "custom", serviceIds?: string[]}`. Omitted defaults to `autodeploy_only`. `custom` requires a non-empty `serviceIds` of linked services the caller may deploy; `none` and `autodeploy_only` refuse `serviceIds`.
- **Response (`201 Created`):** Always includes required `deploys: [{serviceId, deployId}]` and `skippedServices: [{serviceId, reason}]`.
  - `autodeploy_only`: pre-creates awaiting-build deploy rows (`awaiting_build = true`, migration `0341`) for linked services with auto-deploy enabled; skips suspended services silently without listing them in `skippedServices`.
  - `none`: triggers the carrier build only (`deploys: []`, `skippedServices: []`) and fans out nothing on `BuildSucceeded`.
  - `custom`: pre-creates awaiting-build deploys for active named services and lists suspended named services in `skippedServices` with `reason: "suspended"`. `deploys_shut_off` is never emitted because bex has no per-tier deploy shutoff switch.
- **Lifecycle:** Awaiting-build rows promote in place when `BuildSucceeded` runs (retaining `build_deploy_scope` across fan-out retries) and close terminally in the same transaction when the carrier build fails or cancels.
- **GraphQL & MCP:** Mirrored on GraphQL `triggerBuild(..., deploy: BuildDeployScopeInput)` (`Build.deploys`, `Build.skippedServices`) and MCP `trigger_build_source_build`.

### 3. Project-scoped logs: `GET /v1/logs`, `GET /v1/logs/values`, `GET /v1/logs/subscribe`

- **Parameters:** `project` makes `resource` optional and covers the project's services, cron jobs, Postgres, and Key Value instances (skipping static sites when `resource` is omitted). `environment` and `region` each require `project` (400 otherwise). Explicit `resource` narrows `project` and refuses any resource outside the scope (400).
- **Region & `multiple_regions`:** `region` is validated against Render's five regions (`frankfurt`, `ohio`, `oregon`, `singapore`, `virginia`, per w8/415) and normalized to single-region `BEX_REGION`. Because bex runs one physical region, `multiple_regions` never fires and stays in `notApplicableRenderErrorCodes` with updated rationale.
- **Cap:** Exceeding 20 resources returns `400` with `"code": "too_many_resources"` (`savedsearch.TooManyResourcesCode`).
- **Backwards compatibility:** Preserves w3/m505's dashboard project-view tail (`filterDocument` + `textMode`).
- **GraphQL & MCP:** Mirrored on GraphQL `logs`/`logLabelValues` (`environment`, `region`, and `resource` narrowing `project` when `filterDocument` is omitted) and MCP `list_logs`/`list_log_label_values` (bex extensions over Render's pinned MCP tools).

### 4. Project-scoped metrics: all 18 served `GET /v1/metrics/*` operations

- **Operations:** `active-connections`, `bandwidth`, `bandwidth-sources`, `cpu`, `cpu-limit`, `cpu-target`, `disk-capacity`, `disk-usage`, `filters/application`, `filters/http`, `filters/path`, `http-latency`, `http-requests`, `instance-count`, `memory`, `memory-limit`, `memory-target`, `replication-lag`.
- **Parameters:** `project` and `environment` (allowed alone; must belong to `project` when both are provided, else 400) make `resource` optional; `region` requires `project` or `environment` and is validated per w8/415. Explicit `resource` narrows the scope and refuses outside resources (400). Unknown or cross-workspace `project` or `environment` answers `404`.
- **Resource filtering & caps:** When `resource` is omitted, each operation selects only applicable resource kinds (e.g. Postgres only for `replication-lag`, Postgres + Key Value for `active-connections`, web/static for HTTP metrics & filters). Exceeding 50 resources (30 on `bandwidth-sources`) answers `400` with `"code": "too_many_resources"`.
- **GraphQL & MCP:** Mirrored on GraphQL `metrics`/`metricsFilters` (`PROJECT`/`ENVIRONMENT`/`REGION` filter entries) and MCP `get_metrics` (bex extensions).

### 5. No-new-behavior dispositions and retired shims

- `POST /v1/environments/{id}/resources` (`add-resources-to-environment`): now answers `201 Created` (was `200`).
- `subscribe-logs`: dropped `renderOptionalParameterCompatibility["subscribe-logs"]`'s `"resource"` entry and `renderQueryExtensions["subscribe-logs"]`'s `"project"` entry now that Render declares both.
- `create-sandbox-group`: dropped `renderBodyPropertyExtensions["create-sandbox-group"]` now that Render declares `environmentId`; kept a `nullableProperties` compatibility rule for w22/m412 `environmentId: null`.
- `rendertest`: removed the dead `dropUndefinedMetricValueUnit` workaround after Render fixed `values.items.required` to `["timestamp", "value"]`.
- `postgresDetail.project`, `notificationOverride.type`, sandbox `allOf` restructures, and `workflowversions` (Workflows omission): no handler change needed.
