# w8 · m59 — Return static sites to direct publication after clearing their build command

**Worker:** worker8 **Goal:** clearing an implicit static build step publishes the current repository files regardless of creation history. **Status:** todo — sweep184 now passes both fresh build-created native clear transitions; source work is complete. Remaining cross-surface, legacy/strategy-neighbor and complete DoD evidence is actionable.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Normalize implicit static strategy with command intent — **DONE 2026-10-12** | 35m | — |
| t002 | Verify existing sites, shared callers and explicit strategy neighbors — **DONE 2026-10-12** | 30m | t001 |
| t003 | Render parity across REST, GraphQL, MCP and dashboard — native core clear passes sweep184; other surfaces remain | 20m | t002 |
| t004 | Simplify the changed strategy projection — **DONE 2026-10-12** | 10m | t003 |
| t005 | Test creation-history equivalence and command neighbors — code regressions done; sweep184 build-created cases pass; full neighbor/legacy acceptance remains | 30m | t003 |
| t006 | Closeout after complete native and cross-surface acceptance | 10m | t004, t005 |

## Definition of done

- Create a fresh Free static site with the finding's build command, no declared runtime/builder, root `examples/static-site` and publish `dist`. After Live and exact page/asset markers, run `services update ID --build-command '' --publish-directory . --confirm -o json` through released Bex and the actual pinned Render client against Bex, using equivalent owned fixtures. Both saves exit 0 and their resulting deploys become Live; index equals repository content at that deploy's commit and the removed generated `version.txt` returns 404.
- Repeat with a site created for direct publication, then given a build step and cleared. Its final behavior matches the site created with a build step. Verify an already-existing implicitly native site also recovers; fixing only future creates does not satisfy this milestone.
- Omitted commands preserve settings; nonempty commands still run. Explicit runtime, Dockerfile and builder forms retain their documented meaning; web/private/worker/cron command behavior remains correct. Compound build-command/publish-path saves publish one coherent configuration and one rollout.
- Verify every shared exposed setter with truthful attribution, original-content preservation during pending/failed publication, and owned-ID cleanup. Dashboard/MCP/GraphQL writes were not live-tested by the original hunt; their acceptance remains work.

## Progress / remaining work

- **2026-10-13 UTC, sweep184:** [fresh build-created clear acceptance](evidence/live-static-sweep184/README.md) passes both clients with two actual native clear requests/ten HTTP captures, Live/exact source page/generated asset 404 and builder auto readback. Both exact IDs/four children reconciled, baseline6/quota termination0; deployed SHA unknown. Move from blocked/m59 to active m59 and return t003/t005/t006 to todo for the remaining full DoD. Preserve sweep180's historical failure evidence.

- **2026-10-13 UTC, sweep180:** [fresh four-fixture live acceptance](evidence/live-static-sweep180/README.md) captures four native empty-command saves/20 HTTP requests across released Bex and pinned Render. Both sites created with build steps fail after clear and preserve old content; both direct-created→built→cleared controls return Live/source index/asset404. Eight API-only strategy reads corroborate native versus auto builders. Three targeted current-source tests pass; deployed revision unknown. All four exact IDs and ten enumerated child records are reconciled, baseline6/quota termination0. No duplicate issue, product implementation or ship by this hunt.

- **2026-10-12:** `setBuildCommand` (`lego/backend/internal/apps/build_strategy.go`) is the one command write behind `SetCommands` and the patch preflight; clearing a static site's command drops its runtime-less native builder. Operator `directStaticPublish` and backend `buildStrategy` treat an already-cleared native static record as direct publication, so existing sites recover. Regression tests in `buildsettings_test.go` and `app_controller_static_direct_test.go`; backend `go test ./...`, operator `make test`, `make lint` pass.
- **Current remaining work:** native build-created clears now pass. Complete t003 cross-surface writes and t005 legacy/strategy/creation-history acceptance, then close t006 only when the full observable DoD holds. A desired image pin does not establish the live SHA.

## Source + Goal linkage

- **Source:** [researched live CLI finding](finding.md), sweep 99, 2026-10-11. Both native clients accept the clear payload; three sites created with build steps retain native Builder and fail, while the direct-created control returns to Live. All fixtures are cleaned.
- **Goal linkage:** reliable Render-compatible hosting ([ADR018](../../../docs/ADR018-render-parity.md)) and optional static builds ([ADR029](../../../docs/ADR029-static-sites.md)).
- **Expected outcome:** removing a build step deploys the intended plain files rather than trapping a site in an invalid native-build configuration.
- **Why now:** create and update project the same current settings differently, leaving existing sites persistently unable to publish their requested no-build configuration. The related static rollback work does not fix command clearing.
- **Size:** six tasks, 2h15. Shared existing-resource behavior, transport acceptance and meaningful regression coverage make this larger than an inbox note. Render parity is included because user-facing command semantics and shared setters are affected.
- **Boundary:** the original hunt and sweep180 only filed evidence and cleaned QA fixtures. Source implementation and shipping are recorded above as separate work; this QA run did not implement or ship. Render production was not contacted; the official static-site page does not establish the exact empty-command transition.
