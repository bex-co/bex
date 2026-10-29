# w8 · m141 — HTTP-aware Docker web listener selection

**Worker:** worker1 **Goal:** Render's multi-port RabbitMQ example serves its HTTP management interface without PORT. **Status:** done (2026-10-29)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Select a bounded HTTP listener from candidate ports — **DONE** | 35m | — |
| t002 | Gate candidate probes, rollout and digest reuse — **DONE** | 45m | t001 |
| t003 | Persist detected-port tenant narration — **DONE** | 30m | t002 |
| t004 | Render parity — **DONE** | 20m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 30m | t005 |
| t007 | Hosted acceptance — **DONE 2026-10-29** | 30m | t006 |
| t008 | Closeout — **DONE 2026-10-29** | 10m | t007 |

## Definition of done

- A web-image candidate without explicit port/PORT selects the lowest port returning an HTTP status line, including401, before promotion; private and explicit port choices remain unchanged.
- Pending discovery cannot promote a binary TCP listener or drain the prior release; startup scan is bounded and noHTTP uses the recorded fallback.
- NoEXPOSE listeners can be discovered without advertising invented10000; fallback diagnostics truthfully name an empty EXPOSE set.
- A completed decision is reused per digest and matching configured-port input, including later releases. Legacy choices require safe provenance/equivalent fresh metadata, as amended by w8/m142.
- Exact successful deploy history contains Detected HTTP on port N narration, and old deploys cannot borrow another image's choice.
- Hosted RabbitMQ serves15672, nginx serves80 and the noEXPOSE control works; fixtures are deleted by ID with404 proofs. Relevant suites and simplify pass.

## Source + Goal linkage

- **Source:** w8/205, QA sweep5dba2026-10-26; w8/202 noEXPOSE diagnostics follow-up. Promoted2026-10-28.
- **Goal linkage:** Render parity first (ADR008); official Render Docker templates must expose the HTTP interface they promise.
- **Expected outcome:** RabbitMQ's public URL reaches management15672 while AMQP stays private, instead of reporting Live on epmd4369 and returning502.
- **Why now:** image-metadata selection currently applies the private-service rule to public web traffic and the TCP health check falsely passes.
- **Design:** direct bounded HTTP status-line probes after candidate startup, provisional HTTP probes and a150-second replacement hold, per-digest durable selection, existing serving choice retained, fallback after a90-second startup discovery window. The lowest HTTP tie-break is bex policy; Render documents the RabbitMQ outcome but not its algorithm.
- **Render parity:** included for shared REST/GraphQL/MCP/dashboard port projection and tenant deploy-log behavior.

## Verification checkpoint — 2026-10-28

Full operator and types suites, protocol race/socket tests and targeted operator/store/log/API regressions pass. Simplify completed all three reviews, shared the selection/reason constants, reused probe helpers, derived the pending rollout hold and retired completed scan throttle entries. A final operator rerun covers refreshed prior-release availability evidence. Workspace lint/dead-code and fresh-dependency backend ship gates must pass before push. Hosted acceptance remains pending the new operator/API/CRD deployment.

## Parked gate — 2026-10-28

The final full operator suite, types suite, workspace lint/dead-code and fresh-dependency backend ship gate all passed, including migration0172 and exact-release historical port persistence. **BLOCKED:** release pipeline deploys the operator/API/CRD and migration, then QA runs t007: fresh RabbitMQ noPORT reaches management15672 with its historical Detected HTTP narration, nginx remains80, and a noEXPOSE image selects its runtime HTTP port. Confirm explicitPORT/private controls remain unchanged and delete all fixtures by ID with404 proofs. No production fixture was created before the new build was available.


## Subsequent retention correction — 2026-10-28

w8/m142 records configured-port provenance and prevents HTTP selection from retaining a deleted PORT. Its conservative legacy migration supersedes the blanket legacy-choice retention described above: unknown records refresh metadata once and may retain completion only for an exactly matching fresh port tuple. Hosted m141 checks remain pending; this does not claim them complete.


## 解除阻塞记录（2026-10-29，`/unblock w8`）

- **涉及 ID：** w8/m141
- **结论来源：** 本次复核证据 + 现有规则（loopx：线上验收由 worker 用 `scripts/qa-login.sh` / `bex` CLI 自行完成）。
- **复核结果：** 原卡点「发布流水线部署后再做线上 QA」已不成立。生产当前运行 pin `b33dd43c4428`（2026-10-29 06:51 UTC 由 deploy.yml + Argo 自动发布），本条目在 main 上已合入的修复都包含在内。线上验收不需要用户操作。
- **剩余工作：** 上一次线上验收已发现未达标（见上方最近一次 hosted 记录）。这是 worker 能直接修复的实现问题，不需要用户决定。
- **worker 下一步：** 先定位并修复未达标项，补回归测试并 ship；等新修复部署后按原 DoD 复验。fixture 按 ID 删除并留 404 证据。
- **用户门槛：** 无。

## Closeout — 2026-10-29 (`/loopx w8`)

Post-fix re-check on production pin `69f0b17d1fad` (carries `1bafd858f`). Two identical `render-examples/rabbitmq` fixtures (`--runtime docker`, Free, cookie/user/password env, no PORT): `srv-dbhm562kl95s73b458hg` and `srv-dbhm56l5enhs73d3fepg` both logged `==> Detected HTTP on port 15672; serving it` and report `port: 15672`; `GET /` 200; `/api/overview` 401 without and 200 with credentials (`management_version` 4.0.4). nginx:alpine control `srv-dbhm56t5enhs73d3ferg`: `Detected HTTP on port 80`, 200. The no-EXPOSE leg passed on 2026-10-28. All fixtures → 404.
