# w8 · m140 — Password-free internal Key Value connections

**Worker:** worker1 **Goal:** Render's host/port-only Key Value clients work in every workspace without restarting Valkey. **Status:** todo（2026-10-29 解除阻塞，待线上验收；原记录：t001–t007 done; t008 hosted acceptance and t009 closeout pending deployment)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Implement transparent authenticated internal transport — **DONE** | 40m | — |
| t002 | Integrate a readiness-gated stable Service cutover — **DONE** | 50m | t001 |
| t003 | Preserve workspace and environment isolation on both hops — **DONE** | 30m | t002 |
| t004 | Publish Render-shaped internal connection URLs — **DONE** | 25m | t003 |
| t005 | Render parity — **DONE** | 20m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 30m | t006 |
| t008 | Hosted acceptance | 40m | t007 |
| t009 | Closeout | 10m | t008 |

## Definition of done

- Render's webdis guide works with REDIS_HOST/REDIS_PORT alone: PING is PONG and SET/GET round-trip.
- Internal URL is redis://<id>:6379 after readiness-gated migration; old authenticated URLs still work.
- External TLS continues requiring valid authentication; same-environment access works and cross-workspace/environment traffic remains denied.
- CONFIG SET remains denied through the internal front; migration changes no Valkey pod template, preserving Off-cache data.
- Hosted fixtures are deleted by ID with 404 proofs; all relevant suites and simplify pass.

## Source + Goal linkage

- **Source:** w8/204, CLI QA sweep 9098, 2026-10-26; promoted 2026-10-28.
- **Goal linkage:** Render parity first (ADR008); native host/port clients must work against managed Key Value.
- **Expected outcome:** webdis and host/port-only workers reach their private store without a password while external AUTH and default ACL restrictions remain.
- **Why now:** an official Render deployment guide currently fails every command with NOAUTH, and ADR018's unsigned safe-superset claim is false.
- **Design:** a per-store proxy Deployment authenticates upstream as default over a dedicated backend Service. The stable Service switches only after authenticated readiness. External TLS is passed through unchanged. Valkey's template is untouched, avoiding data loss on Persistence Off caches. Internal-auth enforcement toggle is separate follow-up work.
- **Render parity task:** included because shared connection-info and Blueprint URL projections change on every surface.

## Verification checkpoint — 2026-10-28

Protocol race/socket tests, types and Key Value API suites, full operator `make test`, and real-Valkey smoke checks pass. The real Valkey check exercised unauthenticated PING/binary SET/GET, CONFIG denial under the default ACL, old credentialed AUTH, backend auth enforcement and authenticated readiness; its Docker fixture was removed. Simplify completed all three reviews and applied reuse, admission bookkeeping and proxy reservation fixes. Full fresh-dependency backend ship gate and workspace lint must pass before push; hosted acceptance remains pending deployment.

## Parked gate — 2026-10-28

Full operator, types and fresh-dependency backend suites, proxy race tests, real-Valkey smoke check, and workspace lint/dead-code checks passed. **BLOCKED:** the release pipeline must deploy the operator/API image containing `/kv-internal-proxy` and the status-field CRD, then QA must complete t008: webdis host/port PING/SET/GET, old credentialed internal URL, correct/wrong/missing external AUTH, cross-workspace/environment denial and CONFIG denial, with fixture deletion by ID and 404 proofs. Confirm the proxy image pulls in tenant namespaces. Valkey templates remain unchanged during migration. No production fixture was created before this build was deployed.


## 解除阻塞记录（2026-10-29，`/unblock w8`）

- **涉及 ID：** w8/m140
- **结论来源：** 本次复核证据 + 现有规则（loopx：线上验收由 worker 用 `scripts/qa-login.sh` / `bex` CLI 自行完成）。
- **复核结果：** 原卡点「发布流水线部署后再做线上 QA」已不成立。生产当前运行 pin `b33dd43c4428`（2026-10-29 06:51 UTC 由 deploy.yml + Argo 自动发布），本条目在 main 上已合入的修复都包含在内。线上验收不需要用户操作。
- **剩余工作：** 只剩线上验收（待验证，未完成）。部署不等于验收通过。
- **worker 下一步：** loopx 按原 DoD 和上方 BLOCKED 记录里的检查清单在生产执行线上验收：通过则按 done 规则关闭；不通过则修复后复验。fixture 按 ID 删除并留 404 证据。
- **用户门槛：** 无。
