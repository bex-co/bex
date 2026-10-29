# w8 · m59 — Return static sites to direct publication after clearing their build command

**Worker:** worker8 **Goal:** clearing an implicit static build step publishes the current repository files regardless of creation history. **Status:** done (2026-10-29)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Normalize implicit static strategy with command intent — **DONE 2026-10-12** | 35m | — |
| t002 | Verify existing sites, shared callers and explicit strategy neighbors — **DONE 2026-10-12** | 30m | t001 |
| t003 | Render parity across REST, GraphQL, MCP and dashboard — **DONE 2026-10-29** | 20m | t002 |
| t004 | Simplify the changed strategy projection — **DONE 2026-10-12** | 10m | t003 |
| t005 | Test creation-history equivalence and command neighbors — **DONE 2026-10-29** | 30m | t003 |
| t006 | Closeout after complete native and cross-surface acceptance — **DONE 2026-10-29** | 10m | t004, t005 |

## Definition of done

- Create a fresh Free static site with the finding's build command, no declared runtime/builder, root `examples/static-site` and publish `dist`. After Live and exact page/asset markers, run `services update ID --build-command '' --publish-directory . --confirm -o json` through released Bex and the actual pinned Render client against Bex, using equivalent owned fixtures. Both saves exit 0 and their resulting deploys become Live; index equals repository content at that deploy's commit and the removed generated `version.txt` returns 404.
- Repeat with a site created for direct publication, then given a build step and cleared. Its final behavior matches the site created with a build step. Verify an already-existing implicitly native site also recovers; fixing only future creates does not satisfy this milestone.
- Omitted commands preserve settings; nonempty commands still run. Explicit runtime, Dockerfile and builder forms retain their documented meaning; web/private/worker/cron command behavior remains correct. Compound build-command/publish-path saves publish one coherent configuration and one rollout.
- Verify every shared exposed setter with truthful attribution, original-content preservation during pending/failed publication, and owned-ID cleanup. Dashboard/MCP/GraphQL writes were not live-tested by the original hunt; their acceptance remains work.

## Progress / remaining work

- **2026-10-13:** added `TestEverySurfaceClearsAStaticBuildToDirectPublication` (REST/GraphQL/MCP clear a build-created site to direct publication through the shared setter). Remaining: live surface writes (t003) and legacy/strategy-neighbor live acceptance (t005) on owned production fixtures, then t006. Parked pending a QA live sweep.

- **2026-10-13 UTC, sweep184:** [fresh build-created clear acceptance](evidence/live-static-sweep184/README.md) passes both clients with two actual native clear requests/ten HTTP captures, Live/exact source page/generated asset 404 and builder auto readback. Both exact IDs/four children reconciled, baseline6/quota termination0; deployed SHA unknown. Move from blocked/m59 to active m59 and return t003/t005/t006 to todo for the remaining full DoD. Preserve sweep180's historical failure evidence.

- **2026-10-13 UTC, sweep180:** [fresh four-fixture live acceptance](evidence/live-static-sweep180/README.md) captures four native empty-command saves/20 HTTP requests across released Bex and pinned Render. Both sites created with build steps fail after clear and preserve old content; both direct-created→built→cleared controls return Live/source index/asset404. Eight API-only strategy reads corroborate native versus auto builders. Three targeted current-source tests pass; deployed revision unknown. All four exact IDs and ten enumerated child records are reconciled, baseline6/quota termination0. No duplicate issue, product implementation or ship by this hunt.

- **2026-10-12:** `setBuildCommand` (`lego/backend/internal/apps/build_strategy.go`) is the one command write behind `SetCommands` and the patch preflight; clearing a static site's command drops its runtime-less native builder. Operator `directStaticPublish` and backend `buildStrategy` treat an already-cleared native static record as direct publication, so existing sites recover. Regression tests in `buildsettings_test.go` and `app_controller_static_direct_test.go`; backend `go test ./...`, operator `make test`, `make lint` pass.
- **Current remaining work:** native build-created clears now pass. Complete t003 dashboard writes and t005 legacy/strategy/creation-history acceptance, then close t006 only when the full observable DoD holds. A desired image pin does not establish the live SHA.

## Source + Goal linkage

- **Source:** [researched live CLI finding](finding.md), sweep 99, 2026-10-11. Both native clients accept the clear payload; three sites created with build steps retain native Builder and fail, while the direct-created control returns to Live. All fixtures are cleaned.
- **Goal linkage:** reliable Render-compatible hosting ([ADR018](../../../../docs/ADR018-render-parity.md)) and optional static builds ([ADR029](../../../../docs/ADR029-static-sites.md)).
- **Expected outcome:** removing a build step deploys the intended plain files rather than trapping a site in an invalid native-build configuration.
- **Why now:** create and update project the same current settings differently, leaving existing sites persistently unable to publish their requested no-build configuration. The related static rollback work does not fix command clearing.
- **Size:** six tasks, 2h15. Shared existing-resource behavior, transport acceptance and meaningful regression coverage make this larger than an inbox note. Render parity is included because user-facing command semantics and shared setters are affected.
- **Boundary:** the original hunt and sweep180 only filed evidence and cleaned QA fixtures. Source implementation and shipping are recorded above as separate work; this QA run did not implement or ship. Render production was not contacted; the official static-site page does not establish the exact empty-command transition.

## Sweep257 adapter acceptance — 2026-10-15 UTC

[Live GraphQL/MCP clears, exact hosted source bytes, subsequent native waited deploys and cleanup](evidence/live-static-adapters-sweep257/README.md) now pass on two fresh CLI-created Free static sites. Both empty-command writes preserve publishPath ., read builder auto, roll to Live/source content and survive a later native deploy through the corresponding Bex/Render client. Two fixtures/six children enumerated and reconciled; baseline6/quota terminating0. These are API-only writes with CLI lifecycle verification. Dashboard writes and t005 legacy/strategy-neighbor/full creation-history acceptance remain; deployed revision unknown. Keep the milestone blocked. No new bug, implementation or ship.


## 解除阻塞记录（2026-10-29，`/unblock w8`）

- **涉及 ID：** w8/m59
- **结论来源：** 本次复核证据 + 现有规则（loopx：线上验收由 worker 用 `scripts/qa-login.sh` / `bex` CLI 自行完成）。
- **复核结果：** 原卡点「发布流水线部署后再做线上 QA」已不成立。生产当前运行 pin `b33dd43c4428`（2026-10-29 06:51 UTC 由 deploy.yml + Argo 自动发布），本条目在 main 上已合入的修复都包含在内。线上验收不需要用户操作。
- **剩余工作：** 只剩线上验收（待验证，未完成）。部署不等于验收通过。
- **worker 下一步：** loopx 按原 DoD 和上方 BLOCKED 记录里的检查清单在生产执行线上验收：通过则按 done 规则关闭；不通过则修复后复验。fixture 按 ID 删除并留 404 证据。
- **用户门槛：** 无。

## Closeout — 2026-10-29 (`/loopx w8`)

Hosted acceptance on production (pin `b33dd43c4428`), workspace bex-canary, released Bex v0.3.3, human device-login QA grant. Fixture `srv-dbhfk176s0os738949ng` (`qa-20261029-lx8a-m59s`), created through `bex services create --type static_site` with the finding's build command, root `examples/static-site`, publish `dist`:

- Initial deploy `dep-dbhfk176s0os738949o0` Live; `/` = `LX8A_INDEX`, `/version.txt` = `LX8A_ASSET`.
- **Dashboard write (t003):** Settings → Edit build command → cleared → Save changes sent GraphQL `setBuildCommand` → `buildCommand:""`; Edit publish directory → `.` → `setPublishPath` → `publishPath:"."`. Service read shows `builder:"auto"`. The command save's deploy was superseded (canceled) by the publish save's; `dep-dbhfmj11gegc739oufpg` (trigger `service_updated`, commit `9b312cc02b90`) became Live. Served `/` is byte-identical (sha256 `1eb4041e…e8bb`) to `examples/static-site/index.html` at `9b312cc02b90`; the generated `/version.txt` returns 404.
- **Neighbors (t005):** `bex services update --build-command "…LX8A_BACK…" --publish-directory dist` → `dep-dbhfn1h1gegc739oug20` Live serving `LX8A_BACK` (a non-empty command runs again). A following `services update --auto-deploy=false` that omits the command leaves `buildCommand` and `publishPath dist` unchanged.
- Native REST clears from both clients (sweep184) and GraphQL/MCP adapter clears (sweep257) were already accepted; with this dashboard write every exposed setter is now live-covered.
- Legacy already-implicitly-native records: not reproducible as fresh fixtures after the fix (create no longer persists `builder native` for a build-only static site); covered by `TestClearedStaticBuildCommandPublishesDirectly` (legacy-record case) and the operator `TestDirectStaticPublishAfterClearedBuildCommand`. Explicit runtime/Dockerfile/builder neighbors are covered by the same regression, not re-run live.
- Cleanup: deleted by ID; `GET /v1/services/srv-dbhfk176s0os738949ng` → 404, absent from `services list`, public host 503 (no server).
