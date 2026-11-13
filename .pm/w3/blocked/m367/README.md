# w3 · m367 — Render contract round 7: Blueprint cron jobs take `buildSource`, sandbox builds can be deleted, and the pins match Render's 2026-11-12 contracts

**Worker:** worker3 **Goal:** the Blueprint schema, OpenAPI and MCP pins match what Render publishes today. A `render.yaml` that gives a cron job a `buildSource` deploys on bex as it does on Render. `DELETE /v1/sandbox-groups/{id}/builds/{buildId}` deletes an imported image as on Render. The remaining new operations (sandbox-group credentials) are refused by name and recorded. **Status:** blocked (t008: the DoD's production delete check waits on the batched deploy carrying `d8c71580b`; the production pin is batched (`deploy.yml` pins only when the current pin is ≥ 6 h old; the pin was `4c3c68e6b` at 05:33Z, so the next is ~11:33Z 2026-11-13 or later). Then a worker, as the QA user in `bex-canary`, imports a small image into a sandbox group, deletes it (204, gone from list/get/create-by-name, 204 again), and sees a dependent sandbox refuse a delete with 409 `image_in_use` — no user decision needed)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Refresh the Blueprint schema, OpenAPI (263 operations) and MCP pins; record round 7 in ADR018; credentials routes stay named refusals — **DONE** | 45m | — |
| [t002](done/t002.md) | Blueprint cron jobs take `buildSource` (validate, plan, sync, export), and a server's `buildSource` with `rootDir` is refused per Render's schema — **DONE** | 90m | t001 |
| [t003](done/t003.md) | `DELETE /v1/sandbox-groups/{id}/builds/{buildId}` on REST, GraphQL and MCP, as on Render — **DONE** | 60m | t001 |
| [t004](done/t004.md) | Docs: ADR049 build sources, ADR097 cron row, ADR104 builds and credentials — **DONE** | 20m | t002, t003 |
| [t009](done/t009.md) | `POST /v1/build-sources/{id}/builds` takes Render's optional `clearCache` body and clears the build cache as a service deploy does — **DONE** | 45m | t001 |
| [t010](done/t010.md) | Sandbox builds carry Render's nullable `failureCode`: null unless `failed`, a known code when it is — **DONE** | 30m | t001 |
| [t011](done/t011.md) | MCP `list_logs` `task`/`taskRun` (Workflows) recorded as accepted divergences; `connect-sandbox-run` no longer declares a `command` body — **DONE** | 20m | t001 |
| [t005](done/t005.md) | Render parity — **DONE** | 20m | t004, t009, t010, t011 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

## Definition of done

- `bash scripts/render-schema-drift.sh` and `bash scripts/render-mcp-drift.sh` report no drift against Render's live contracts.
- A `render.yaml` with a `buildSources` entry and a `type: cron` service that sets `buildSource:` and no `runtime` validates, plans and syncs on dev-3. The cron job runs the build source's image on schedule, and the Blueprint export round-trips the same YAML.
- A cron or server service that sets `buildSource` together with a field Render's schema forbids beside it (`runtime`, `repo`, `branch`, `image`, `registryCredential`, `buildCommand`, `buildFilter`, `dockerContext`, `dockerfilePath`, `rootDir`) is refused with the schema path in the error.
- On production, after the deploy, deleting an imported sandbox build answers 204. The build is gone from the list and get-by-id (404), a sandbox create by its id or image name is refused, and a repeated delete answers 204. Its registry image is deleted.
- The seven sandbox-group credential operations answer the named `SANDBOX_CREDENTIALS_UNSUPPORTED` refusal and are listed as omissions.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-11-12, item 3, filed by `/pmx them all to w3`. Re-checked 2026-11-12 against `origin/main` `24bea6c5c` and Render's live contracts.
- **Evidence (live vs pin, 2026-11-12):**
  - OpenAPI: Render serves **263** operations; the pin (`lego/backend/internal/api/openapi/render-public-api-1.json`, round 6, 2026-11-08) has 255. New: `delete-sandbox-build` and seven `sandbox-groups/{sandboxGroupId}/credentials*` operations (list, create, retrieve, update, delete, list versions, restore). None removed.
  - Render's `delete-sandbox-build`: "Delete a build of a sandbox image. The build no longer appears in lists and cannot be retrieved or used to create sandboxes, including by image name. Repeated requests to delete the build return HTTP 204." Responses 204/400/401/403/404/409/429/500/503.
  - Blueprint schema (`https://render.com/schema/render.yaml.json` vs `lego/backend/internal/apps/schema/render.yaml.json`): `cronService` gains `buildSource`, `required` drops `runtime` (now `type, name, schedule`), and an `allOf` forbids the repo/image/build fields beside `buildSource`. `serverService`'s forbidden list gains `rootDir`.
  - bex's compiler closes the schema's objects (`lego/backend/internal/apps/blueprint_compiler.go`, `closeBlueprintSchemaObjects`), so Render's valid cron-with-`buildSource` is refused as an unknown field today.
  - Render MCP added Workflows tools (render-mcp-server #189, 2026-11-11); Workflows are a deliberate `—` non-goal (DO_NOT_DO).
- **bex today:** w2/m358 (done 2026-11-12) serves create, list and get of sandbox builds (`lego/backend/internal/sandbox/images.go`, `images_surfaces.go`), and w2/m359 deletes a group's imported images through `RegistryReaper.DeleteImportedImage` (`sandbox/snapshot_size.go:332`). No route serves `DELETE …/builds/{buildId}`. w3/m270 (done) shipped `buildSources` and server `buildSource`.
- **Goal linkage:** ADR008 Render parity first, on Blueprints and the sandbox API; ADR018's contract-pin discipline (w2/m251, w6/m96).
- **Expected outcome:** Render templates that build a cron job from a shared build source deploy unmodified; a sandbox image can be removed without deleting its group; the weekly drift job is green again.
- **Why now:** the weekly `render-schema-drift.yml` will open an issue on 2026-11-16 for the same diff, and it only detects. The cron change tightens the contract w3/m270 implemented days ago, and the build delete completes w2/m358's surface.
- **Open decision, recommendation only (not a user decision):** sandbox-group credential brokering stays a named refusal. It is now in Render's public OpenAPI, but render.com/sandboxes still labels brokering Early access (checked 2026-11-12), and ADR104 puts Early-access features outside the parity bar until they leave it. The user may decide to build it earlier.
- **Render parity included:** Blueprint, REST, GraphQL and MCP.

## Decision (2026-11-13, `/loopx w3`, triage)

- **Evidence:** the full live-vs-pin diff, run with the drift scripts and a component- and operation-level comparison, found three deltas beyond the brainstorm's list.
  1. `trigger-build-source-build` gained an optional `clearCache` body (t009).
  2. Sandbox builds gained a nullable `failureCode` (t010).
  3. `connect-sandbox-run` dropped its `command` body, and MCP `list_logs` gained the Workflows `task`/`taskRun` filters (t011).
- `delete-sandbox-build`'s 409 is `image_in_use`: sandboxes that have not terminated or errored, or unexpired snapshots, still depend on the build. bex has no snapshots, so that means running or suspended sandboxes (t003). `errorCode` gained `image_in_use`.
- The other changed operations need no handler change:
  - `429RateLimit` and sandbox error responses were restructured or redescribed.
  - The snapshot `name` filter and `ttlSeconds` apply to snapshots, which bex refuses by name (`sandbox/notoffered.go:50-54`, w32/m285).
- **Tasks:** t009–t011 were added before the closing tasks, and t005 now depends on them. Total ~5h45m.
