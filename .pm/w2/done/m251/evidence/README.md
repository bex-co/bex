# w2/m251 evidence — Render drift round 5 (2026-11-02)

## Sources

- Live OpenAPI: `https://api-docs.render.com/openapi/render-public-api-1.json`, fetched 2026-11-02, sha256 `6ac5d3d473625d5e8d921e3a008f500afa400c1f14e25c1f55efc657f6fdbf44` (432,906 bytes). Previous pin: 2026-09-11, sha256 `c6851bc8…` (345,074 bytes).
- MCP: `render-schema-drift` workflow run 41478883290 (2026-11-02) — job `render-mcp` **success** (upstream `tools/list` matches `render-mcp-tools.json`); job `render-schema` failure on the webhook enum only (`+disk_snapshot_restored`, red since run 40328897751, 2026-10-26).
- CLI: `render-oss/cli` `v2.29.0..v2.35.0` (74 commits, upstream `CHANGELOG.md`).

## t001 — operations and schemas

- **Operations:** 252 live vs 208 pinned; **44 added, 0 removed**: `Build Sources` 16 (`/build-sources` CRUD, `/builds` list/trigger/get, `/env-vars` ×5, `/secret-files` ×5, `/env-groups/{envGroupId}` link/unlink); `Workflow Flows` 8 (`/flows`, `/flow-runs`, steps); `Sandboxes` 20 (`/sandboxes` create/list/get/terminate/execs/snapshots/run and file connect tokens, `/sandbox-groups` CRUD and snapshots).
- **Changed schemas on 32 existing operations** (field-level, refs resolved, descriptions ignored):
  - services (`list`, `retrieve`, `create`, `update`, `preview`): `autoDeployTrigger` (`off|commit|checksPass`), `autoDeploy` deprecated, `buildSourceId`; `servicePOST` details no longer require `runtime` (a build source supplies it).
  - deploys (`list`, `retrieve`, `create`, `cancel`, `rollback`): `buildId`; create-deploy request `buildId`.
  - blueprints: `ownerId` required on responses; `update-blueprint` request `branch`; `list-blueprint-syncs` `cursor`/`limit`.
  - postgres (`list`, `retrieve`, `create`, `update`, `recovery`): `parameterOverrides` removed (schema `postgresParameterOverrides` deleted).
  - webhooks / events: enum `+disk_snapshot_restored`; event details "Disk Snapshot Restored" `{diskId, instanceId?, snapshotCreatedAt, triggeredByUser}` (the 300 index-level diffs under `GET /events/{eventId}` are this one oneOf insertion shifting positions). Already in the old pin and unserved by bex: `service_disk_usage_high`/`_recovered` (`{diskId, diskName, mountPath, usagePercent, thresholdPercent}`).
  - audit logs: `+UpdateWorkspacePaymentMethodEvent`, `+UpdateOrgPaymentMethodEvent`, `+5` Build Source events, `−CreateArtifactSourceEvent`.
  - `errorCode`: `+build_not_found`, `+build_not_succeeded`, `+build_source_region_mismatch`, `+insufficient_token_scope`, `+invalid_sandbox_plan`, `+invalid_timeout_seconds`, `+sandbox_group_limit_reached`, `+sandbox_group_suspended`.
  - task-runs (Workflows, non-goal): 400 bodies, `idempotencyKey`.
- **Schemas:** +6 (`*WithCursor` for flows, flow runs, executions, sandboxes, sandbox groups, snapshots), −1 (`postgresParameterOverrides`).

## t002 — MCP and changelog

- MCP tool set unchanged (drift job above). Changelog items since 2026-09-10 relevant to hosting are the Build Sources, Blueprint onboarding (CLI) and sandbox GA items already covered above.

## t003 — CLI v2.29.0 → v2.35.0

Recorded in `docs/cli-compatibility-checklist.md` ("Upstream surface delta v2.29.0 → v2.35.0"). Hosting gap: `ea blueprints create|plan|approve|update` (public alpha, v2.35.0) → **w2/m256**.

## t004 — pin refresh

- Found while refreshing: Render's `sandboxId` pattern `^sbx-[0-9a-v]{25}$` would 400 every bex sandbox id (`sbx-` + 20) and `list-sandboxes` requires `ownerId`; probe through the strict gate showed 400 `invalid path parameter "sandboxId"` on get/terminate/run/file token and `invalid query parameter "ownerId"` on list. Widened (superset) and pinned by `TestSandboxCallsPassTheRenderGate`.
- Build Sources: 20 routes answer 404 `bex does not offer Render Build Sources yet`; non-empty `buildSourceId` refused by name on create (serviceDetails) and PATCH (top level).
- Weekly drift job now also compares the REST operation list (`scripts/render-schema-drift.sh`, 5 new self-tests); live run: Blueprint schema matches, 68-value webhook fixture matches, 252 operations match.
