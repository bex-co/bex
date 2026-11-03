# w2 · m254 — Build Sources: one build feeding several services, as on Render

**Worker:** worker2 **Goal:** bex serves Render's `/build-sources` contract (git- or image-backed, build-time env, linked services, builds list and trigger) and `buildSourceId` on service create, PATCH and GET, on every surface. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Design ADR: Build Sources on bex's CR and build model | 1h | w2/m251/t004 |
| t002 | Types, storage and build fan-out | 2h | t001 |
| t003 | REST CRUD for /v1/build-sources | 1.5h | t002 |
| t004 | Build-time env vars, secret files and env-group links | 1.5h | t003 |
| t005 | buildSourceId on service create and PATCH | 1h | t003 |
| t006 | Trigger a build and list builds | 1h | t004, t005 |
| t007 | GraphQL and MCP equivalents | 1h | t006 |
| t008 | Dashboard: Settings → Build → Source "Update source" dialog | 1.5h | t007 |
| t009 | Render parity | 15m | t008 |
| t010 | Simplify | 15m | t009 |
| t011 | Test coverage | 30m | t009 |
| t012 | Closeout | 10m | t010, t011 |

## Definition of done

- On production, the pinned Render CLI or REST creates a git-backed build source, links two services to it, and one triggered build deploys both.
- `buildSourceId` round-trips on `GET/POST/PATCH /v1/services`; create with `buildSourceId` plus `repo`/`branch`/`image`/`runtime` is rejected as Render does, and a region mismatch returns `build_source_region_mismatch`.
- Build-time env vars, secret files and env-group links on a build source work through REST, GraphQL and MCP; the dashboard Settings → Build → Source dialog switches a service's build source.
- ADR018's Build Sources row is `✅` (or `◐` with each divergence signed off).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-11-02 #4; live OpenAPI 2026-11-02 (tag `Build Sources`, 16 operations; `buildSourceId` on `service`, `servicePATCH`, `webServiceDetailsPOST`, `privateServiceDetailsPOST`, `backgroundWorkerDetailsPOST`, `cronJobDetailsPOST`). Render dashboard flow: service Settings → Build → Source → Edit → "Update source" dialog.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first).
- **Expected outcome:** monorepo/multi-service users build once and deploy many, exactly as on Render; Render clients that send `buildSourceId` work against bex.
- **Why now:** the field is on Render's core service object; Render clients will read and write it. Depends on w2/m251's pin refresh.
- **Render parity included:** new REST/GraphQL/MCP/UI surface.
