# w2 · m262 — Build Sources follow-ups: dryRun, picker paging, linked-service settings, linked crons, deploy provenance

**Worker:** worker2 **Goal:** close the small gaps m254's reviews and parity pass left on Build Sources, so every surface and every linked service type behaves as on Render. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | dryRun on GraphQL linkBuildSource and MCP link_service_to_build_source — **DONE** | 15m | — |
| t002 | Update Source picker pages past 100 build sources — **DONE** | 30m | — |
| t003 | A linked service's Settings shows the right Deploy card — **DONE** | 45m | — |
| t004 | A cron job linked to a native build source runs under native-init in bash — **DONE** | 45m | — |
| t005 | Fan-out deploys carry the build's commit — **DONE** | 30m | — |
| t006 | Render parity — **DONE** | 15m | t001, t002, t003, t004, t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 30m | t006 |
| t009 | Closeout — **DONE** | 10m | t007, t008 |

## Definition of done

- GraphQL `linkBuildSource(dryRun: true)` and MCP `link_service_to_build_source` with `dryRun` run every check and write nothing, as REST PATCH `?dryRun` does.
- The Update Source picker reaches every build source in a workspace with more than 100.
- A service linked to a build source shows a Deploy card that matches it: no image Deploy card for a git source.
- A cron job linked to a native build source runs its command as bash arguments to the image's native-init entrypoint, like a linked web service (w2/m254 t017).
- A deploy opened by a build fan-out carries the build's commit (id and message), as Render's deploy object does.

## Source + Goal linkage

- **Source:** w2/071, promoted 2026-11-03. It comes from w2/m254 t009 (parity) and the m254 t008/t013/t017 reviews. The proposed dashboard create page was dropped: Render documents build sources only through its API (api-docs.render.com create-build-source), with no dashboard create or list page.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); [ADR097](../../../docs/ADR097-build-sources.md).
- **Expected outcome:** linking, listing and deploying from a build source behaves the same on REST, GraphQL, MCP and the dashboard, for every service type.
- **Why now:** Build Sources just shipped (m254), and these gaps are visible to anyone using it beyond the basic path.
- **Render parity included:** it touches GraphQL, MCP and the dashboard.
