# w8 · m59 — Return static sites to direct publication after clearing their build command

**Worker:** worker8 **Goal:** clearing an implicit static build step publishes the current repository files regardless of creation history. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Normalize implicit static strategy with command intent | 35m | — |
| t002 | Verify existing sites, shared callers and explicit strategy neighbors | 30m | t001 |
| t003 | Render parity across REST, GraphQL, MCP and dashboard | 20m | t002 |
| t004 | Simplify the changed strategy projection | 10m | t003 |
| t005 | Test creation-history equivalence and command neighbors | 30m | t003 |
| t006 | Closeout after native and runtime acceptance | 10m | t004, t005 |

## Definition of done

- Create a fresh Free static site with the finding's build command, no declared runtime/builder, root `examples/static-site` and publish `dist`. After Live and exact page/asset markers, run `services update ID --build-command '' --publish-directory . --confirm -o json` through released Bex and the actual pinned Render client against Bex, using equivalent owned fixtures. Both saves exit 0 and their resulting deploys become Live; index equals repository content at that deploy's commit and the removed generated `version.txt` returns 404.
- Repeat with a site created for direct publication, then given a build step and cleared. Its final behavior matches the site created with a build step. Verify an already-existing implicitly native site also recovers; fixing only future creates does not satisfy this milestone.
- Omitted commands preserve settings; nonempty commands still run. Explicit runtime, Dockerfile and builder forms retain their documented meaning; web/private/worker/cron command behavior remains correct. Compound build-command/publish-path saves publish one coherent configuration and one rollout.
- Verify every shared exposed setter with truthful attribution, original-content preservation during pending/failed publication, and owned-ID cleanup. Dashboard/MCP/GraphQL writes were not live-tested by the original hunt; their acceptance remains work.

## Source + Goal linkage

- **Source:** [researched live CLI finding](finding.md), sweep 99, 2026-10-11. Both native clients accept the clear payload; three sites created with build steps retain native Builder and fail, while the direct-created control returns to Live. All fixtures are cleaned.
- **Goal linkage:** reliable Render-compatible hosting ([ADR018](../../../docs/ADR018-render-parity.md)) and optional static builds ([ADR029](../../../docs/ADR029-static-sites.md)).
- **Expected outcome:** removing a build step deploys the intended plain files rather than trapping a site in an invalid native-build configuration.
- **Why now:** create and update project the same current settings differently, leaving existing sites persistently unable to publish their requested no-build configuration. The related static rollback work does not fix command clearing.
- **Size:** six tasks, 2h15. Shared existing-resource behavior, transport acceptance and meaningful regression coverage make this larger than an inbox note. Render parity is included because user-facing command semantics and shared setters are affected.
- **Boundary:** filing only. Implementation, platform deployment and shipping were not performed. Render production was not contacted; the official static-site page describes the lifecycle but does not establish the empty-command transition.
