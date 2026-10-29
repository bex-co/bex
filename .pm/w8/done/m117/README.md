# w8 · m117 — Canceling the newer of two open deploys strands the older

**Worker:** worker8 **Goal:** Canceling one deploy never strands another: when two deploys are open for a service and the newer (latest-pending) one is canceled, the older open deploy is dispatched and reaches a terminal state on its own merits. **Status:** done (2026-10-29)

## Tasks (in order)

| id   | title                                                                | est | depends_on     |
| ---- | -------------------------------------------------------------------- | --- | -------------- |
| t001 — **DONE** | Diagnose the stranded-row mechanism and choose the fix             | 45m | —              |
| t002 — **DONE** | Fix cancel/adoption so the surviving open deploy proceeds          | 90m | w8/m117/t001   |
| t003 — **DONE** | Render parity: cancel semantics across REST, GraphQL, MCP, and UI  | 30m | w8/m117/t002   |
| t004 — **DONE** | Simplify the changed code                                          | 20m | w8/m117/t003   |
| t005 — **DONE** | Test coverage: cancel-newer, cancel-older, and supersede interplay | 50m | w8/m117/t003   |
| t006 | Close out the milestone — **DONE 2026-10-29** | 10m | w8/m117/t005   |

Six tasks, 245m.

## Definition of done

- Live repro passes: create a free web service (initial deploy A open), trigger deploy B, cancel B via `bex deploys cancel` — A leaves `created`/`queued` within 5 minutes and reaches `live` (or honestly fails its build/rollout); the service URL serves 200. B stays `canceled`.
- The same holds for an established service: canceling the newest pending deploy leaves the previously open deploy (or the live release) unaffected and advancing; no deploy row sits in `created`/`queued` with an untouched `updatedAt` for > 10 minutes while a same-shape control service builds normally.
- Canceling the older deploy still supersedes to the newer exactly as today (sweep-51 behavior preserved), and canceling the only open deploy still settles to the last served release / `PhaseCanceled`.
- Regression tests cover cancel-newer-with-older-open at the store/reconciler level and pass in CI.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` sweeps 52–53, 2026-10-18 (this loop's ledger `/tmp/bex-qa-cli-loop/ledger.txt` at filing time). Repro ×2 against `https://api.bex.co/v1/` in workspace bex-canary with the pinned-CLI HEAD build: sweep 52 older deploy sat in `created` ~9.5m until the fixture was deleted; sweep 53 (`srv-dba8idfarj3c73bslk6g`, older `dep-dba8idfarj3c73bslk70` created 08:54:13Z, newer `dep-dba8ifnarj3c73bslk8g` canceled 08:54:22Z) sat in `queued` with `updatedAt` untouched since 08:54:17Z for 13m+, while a same-shape control service created 09:05 reached `build_in_progress` in ~80s — so this is not build-capacity queueing.
- **Goal linkage:** deploys are the core hosting primitive (ADR008 Render-alternative core); a cancel — an explicitly per-deploy Render API (`POST /services/{id}/deploys/{deployId}/cancel`) — silently wedging a different deploy breaks the deploy contract and, for a brand-new service, leaves it never-live with no error.
- **Expected outcome:** per-deploy cancel is per-deploy; overlapping-deploy state machine has no stranded-open-row state.
- **Why now:** found live twice in one QA day with a trivial two-command repro any API/CLI/dashboard user can hit (double-trigger then cancel the duplicate). The stranded row only closes via the gate-timeout fallback (~35m), if at all, so users see a deploy that never starts and no failure reason.
- **Render parity included because** cancel is a tenant-facing verb on REST/GraphQL/MCP and the dashboard; the fix must not change cancel's wire semantics (200/409 shapes) on any surface.


## 解除阻塞记录（2026-10-29，`/unblock w8`）

- **涉及 ID：** w8/m117
- **结论来源：** 本次复核证据 + 现有规则（loopx：线上验收由 worker 用 `scripts/qa-login.sh` / `bex` CLI 自行完成）。
- **复核结果：** 原卡点「发布流水线部署后再做线上 QA」已不成立。生产当前运行 pin `b33dd43c4428`（2026-10-29 06:51 UTC 由 deploy.yml + Argo 自动发布），本条目在 main 上已合入的修复都包含在内。线上验收不需要用户操作。
- **剩余工作：** 上一次线上验收已发现未达标（见上方最近一次 hosted 记录）。这是 worker 能直接修复的实现问题，不需要用户决定。
- **worker 下一步：** 先定位并修复未达标项，补回归测试并 ship；等新修复部署后按原 DoD 复验。fixture 按 ID 删除并留 404 证据。
- **用户门槛：** 无。

## Closeout — 2026-10-29 (`/loopx w8`)

Final re-check on production pin `2ac6fd95402e` (carries all three m117 fixes: `e224872d5`, `6a2338b63`, `2ac6fd954`), bex-canary. echo-server 0.9.2, Free, no PORT (image-ports mode); A created over REST, B with `bex deploys create`, B canceled with `bex deploys cancel` 0.5–2 s later; App CR watched read-only every ~1 s.

| Run | Service | Result |
| --- | --- | --- |
| New service 1 | `srv-dbhrlfpa6jns73b2m47g` | A `live` in 55 s, URL 200, B `canceled`; CR Deploying → Running at `rev-1`, never Failed |
| New service 2 | `srv-dbhrm0m9e4qs73fgran0` | A `live`, URL 200, B `canceled`, never Failed (A waited ~11 min for tenant memory quota freed by deleting run 1 — filed as w8/223) |
| Established 1 | same service after a clean live deploy | A `live`, B `canceled`, Running at A's revision; concurrent 0.25 s poller 236/236 × 200 |
| Established 2 | same | A `live`, B `canceled`; poller 178/178 × 200 |
| Repo-backed control | `srv-dbhrtb1a6jns73b2m5c0` (render-examples/express-hello-world) | A built and went `live`, URL 200, B `canceled` |

Cleanup: all three services deleted by ID (`GET` → 404, no App CR, URLs 404).
