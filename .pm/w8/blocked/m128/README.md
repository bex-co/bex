# w8 · m128 — `dockerContext` is relative to `rootDir`, as on Render

**Worker:** worker8 **Goal:** Render's monorepo Docker shape (`rootDir: backend`, `dockerContext: .`, `dockerfilePath: ./Dockerfile`) builds on bex unchanged, every surface reads the context back the way Render reports it, and the context can be changed in place. **Status:** blocked — t001–t008 done 2026-10-20; t009 live check on production after the release pipeline deploys bex-api and the dashboard

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 — **DONE** | Contract: dockerContext is relative to rootDir (ADR006/ADR049, CRD and MCP copy) | 20m | — |
| t002 — **DONE** | Writes: create and Blueprint store join(rootDir, dockerContext) | 30m | w8/m128/t001 |
| t003 — **DONE** | Reads: REST, GraphQL, MCP and the Blueprint export report the rootDir-relative value | 20m | w8/m128/t002 |
| t004 — **DONE** | rootDir changes rebase a stored dockerContext | 15m | w8/m128/t002 |
| t005 — **DONE** | PATCH dockerContext (from w8/151): REST, MCP and GraphQL | 30m | w8/m128/t004 |
| t006 — **DONE** | Render parity: dockerContext in ADR018 | 10m | w8/m128/t005 |
| t007 — **DONE** | Simplify the changed code | 15m | w8/m128/t006 |
| t008 — **DONE** | Test coverage: build paths, read-back, rebase, PATCH, round trip | 30m | w8/m128/t006 |
| t009 | Close out the milestone | 30m | w8/m128/t007, w8/m128/t008 |

## Definition of done

Live on production after the release pipeline deploys bex-api, on fresh free fixtures deleted by ID with 404 proofs:

- A Docker service created with `rootDir: example-pserv`, `dockerContext: "."` (sweep 1266's `render-examples/twingate-example` shape) builds; `COPY index.html` → `DONE`.
- The control without `dockerContext` reads back `dockerContext: "."` on REST, GraphQL and MCP, and its Blueprint export round-trips.
- `PATCH /v1/services/{id}` with `envSpecificDetails.dockerContext` → 200, and the next deploy builds from the new context.

## Source + Goal linkage

- **Source:** inbox notes `w8/150` and `w8/151` (`/qa-find-bugs-cli` sweep 1266, 2026-10-20, [evidence](../../evidence/cli-sweep1266/README.md)), kept in [done/150.md](../../done/150.md) and [done/151.md](../../done/151.md). Promoted 2026-10-20: about 2h across five surfaces, over the inbox size.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md) Render alternative; [ADR018](../../../../docs/ADR018-render-parity.md) docker settings; [ADR049](../../../../docs/ADR049-render-yaml-parity.md) `render.yaml` parity. Render's schema: `dockerContext` is "relative to rootDir"; Render's monorepo guide lists the Docker build context among the settings that operate relative to the root directory.
- **Expected outcome:** a team moving a Render monorepo gets a working first build instead of `"/requirements.txt": not found`, and a wrong context can be repaired without deleting the service.
- **Why now:** it breaks Render-exported monorepo Docker services on import, a major migration path.
- **Render parity included:** REST, MCP, GraphQL and Blueprint semantics change.
- **Decision (fix option 2):** storage stays repo-root (`spec.dockerContext`, empty = `rootDir`) and the translation sits at the API boundary, so no live App CR is rewritten and every existing build is byte-identical; a `rootDir` change rebases the stored context. Blueprint files written to bex's old repo-root reading (`rootDir: x`, `dockerContext: x`) now mean `x/x`, as on Render.

## Progress (2026-10-20)

- t001: ADR006 and ADR049 paragraphs, the CRD field comment (storage meaning unchanged, CRD YAML regenerated) and MCP copy say `dockerContext` is relative to `rootDir`.
- t002: `specFromCreate` stores `storedDockerContext(rootDir, wire)` (`internal/apps/docker_context.go`), covering REST, MCP and Blueprint create; a context equal to `rootDir` stores empty. The Blueprint `dockerContext` applier resolves against the service's effective `rootDir`, which matters when the file omits `rootDir` (preserved on omission).
- t003: `view()` (`viewDockerContext`), `render.go` (`cmp.Or(…, ".")`) and the Blueprint export report the rootDir-relative value; export omits the default `.`.
- t004: `SetRootDir`, the PATCH probe and the Blueprint `rootDir` applier rebase a stored context (`rebaseDockerContext`).
- t005: `SetDockerContext` on the shared patch table after `RootDir` (REST `envSpecificDetails.dockerContext`, MCP `update_service.dockerContext`, GraphQL `setDockerContext`), audit `apps.SetDockerContext`, event `docker_context_changed` (dashboard catalog + filter label, en/zh). Shared `checkDockerBuildSetting` gives the Dockerfile-build-only refusal; `checkDockerContextPath` is create's and update's one path rule.
- t006: ADR018 "Runtime, build strategy + Root Directory" row.
- t007: `/simplify` applied (shared checks; three Blueprint/probe rebase bugs the review found, now under test).
- t008: `docker_context_test.go` (translation, rebase, create read-back incl. sweep 1266's shape, REST shape, export, `SetRootDir`, PATCH success/both-fields/refusals/dry-run, Blueprint appliers), updated `TestValidateBlueprintDockerContext` to the rootDir-relative meaning, guard tables (`settings_test.go`, `patch_fold_test.go`, `create_owned_fields_guard_test.go`).

