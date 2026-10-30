# w8 · m68 — Preserve executable command text

**Worker:** worker8 **Goal:** accepted build/start/pre-deploy/cron commands execute with the argument bytes the user authored. **Status:** blocked — [upstream PR #75](https://github.com/render-oss/cli/pull/75) 已提交；等 upstream merge/release、Bex delivery 和原线上验收

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Preserve nonblank command flags in an upstream-ready change — **DONE** | 35m | — |
| t002 | Preserve command text in shared backend writers — **DONE** | 45m | — |
| t003 | Preserve command drafts in dashboard producers — **DONE** | 35m | — |
| t004 | Render parity — **BLOCKED: PR #75 submitted; upstream merge/release + Bex delivery + original acceptance** | 25m | t001, t002, t003 |
| t005 | Simplify — **DONE** | 10m | t004 |
| t006 | Test coverage — **DONE** | 30m | t004 |
| t007 | Closeout — **BLOCKED: PR #75 submitted; upstream merge/release + Bex delivery + original acceptance** | 15m | t005, t006 |

## Definition of done

- Native create/update for all four command flags preserve nonblank executable text; escaped terminal space/tab reaches the launched process. The actual hosted argument must be hex7461696c20 for the recorded space example, through fresh Bex and unmodified upstream journeys.
- Shared backend and dashboard producers preserve that text; REST/GraphQL/MCP/readback agree. Nil/omit, explicit blank clears, create-required validation, static build clearing and cron legacy-fallback/role behavior remain correct. Generic metadata/path normalization is unchanged.
- Maintained regressions and required affected checks pass. Any deliberate command-padding policy expectation change is explicit. Upstream external submission/adoption, published Bex release and backend/dashboard rollout are separately authorized and verified before claiming customer repair.
- Every owned fixture/child/session created for acceptance is cleaned; immutable baseline and quota are reconciled. Complete all tasks and move the milestone and tasks to done with synchronized statuses and no stubs.

## Source + Goal linkage

- **Source:** continuous functional `$qa-find-bugs-cli` w8/sweep284; [finding](finding.md), [durable evidence](evidence/cli-sweep284/README.md).
- **Goal linkage:** ADR008 usable hosting, ADR018 Render-compatible command execution, ADR004 deployment/startup and ADR049 Blueprint command fidelity.
- **Expected outcome:** a valid escaped shell argument survives CLI, API, stored configuration and launch.
- **Why now:**32 native local captures alter accepted commands and two Free production services actually run the wrong argument while reporting success. Independent backend writers and source-established dashboard producers can reintroduce it even after a client fix.
- **Sizing:**195m / seven tasks across upstream CLI, backend, dashboard, parity and observable released acceptance. This exceeds an inbox note.
- **Render parity:** included because this affects tenant-facing CLI/API/UI command semantics. Actual Render production is untested; unchanged exact-pin and released2.29 builds establish the upstream boundary.

Severity minor. The private upstream/backend diagnostics are research material; QA has landed no product fix, dependency bump, deployment, commit/push or external submission. Two owned parents/two children are removed, baseline6/quota6/terminating0 verified. Browser/shared live writes and fixed-release acceptance remain open.

## Current-source and remaining live pre-deploy bytes — sweep299

[Focused source/live evidence](evidence/cli-sweep299/README.md) verifies41 named source-test passes at f497ddab6, but an actual raw API-created Free Node fixture still stores a shortened pre-deploy command and executes hex7461696c5c. Build/start controls execute the authored hex7461696c20. Both actual native clients read the same one program output from build history and exit0. Exact deployed revisions are unobserved; this is the existing live defect, not proof of a source regression or a new deploy-lag issue. No compound update/manual-deploy or fixed-release acceptance is claimed after this failure.

Fresh upstream main remains28b233e, with the six inspected hosting files byte-identical to the pin. Upstream authorization/release/adoption and actual deployed backend/dashboard acceptance remain separate gates. The own Free parent/one child are fully cleaned, baseline6/quota6/terminating0. QA changed no product source and performed no external submission, release or new ship.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m68；D2 上游提交批次共 19 项，完整映射见 workstream README 本次复核表。
- **结论来源：** 本次只读 PR 去重和原已准备补丁记录。2026-10-29 `render-oss/cli` 最近 100 条全状态 PR 返回 28 条；没有发现本批补丁的对应提交，#54–#57 仍 open。PR 索引不是最新源码已修复/未修复的充分证据。 本次还对全部 19 份补丁执行 `git apply --check`，均可应用于当前 pin `af9bdd180e90`（Render v2.29.0）；这不是最新 upstream main 已审核或完整测试通过的声明。
- **剩余门槛：** D2 待用户明确允许以其 GitHub 身份公开提交这些已准备的最小补丁；本次没有对外消息、PR、push 或跟进。提交前由 worker 逐项重查 upstream main / issue、刷新与重测补丁；已被上游修复的应按证据收敛，不重复提交。
- **后续：** 即使允许提交，也仍需 upstream 接受/发布、Bex 审阅 pin 更新及单独授权 CLI release，之后逐项执行原 native / shared-surface / cleanup 验收。不 fork、不修请求来掩盖客户端缺陷。本项继续 blocked，不把允许提交当成已合并或已交付。

## 用户批准 D2（2026-10-29，提交对 Render CLI 有帮助的修复）

- **来源：** 用户明确回复「D2 approve to submit changes that are helpful」。此许可适用于已列出的 19 项，提交确实仍对上游有帮助的修复，不是无条件制造 19 个 PR。
- **授权范围：** 在独立 upstream checkout 审核最新 main、去重、刷新补丁、补行为测试；必要的 fork 分支 commit/push 和向 render-oss/cli 提交 PR 是本次明确授权提交的组成部分。不修改/发布 bex 本仓库代码，不给 bex main 新增 commit/push 权限，不重复询问已给出的提交许可。
- **判定：** 已修复或重复的附证据跳过；可证实通用缺陷且验证充分的提交最小修复；没有通用依据的保留说明，不把 bex 特有差异强推上游。公开内容只含通用复现、修复及测试，不带租户、凭据或生产私有日志。
- **剩余：** 每项记录独立的 PR 链接、上游基线、测试/限制；提交不等于合入或发布，原 adoption / native / cleanup DoD 保留，仍受真实 upstream merge/release 门槛约束。

## D2 已提交上游（2026-10-29，PR #75）

- **结论来源：** 用户明确批准 D2 + 最新 upstream 源码复核、行为测试和本次实际提交。
- **涉及 ID：** w8/m68, w8/108。[Preserve explicit service input during normalization](https://github.com/render-oss/cli/pull/75)，head `d391ee034a4eda43a55e0db187f050e8bbc8c506`，base `c31e586eb6f38138af3cc986b079e46a78c61de7`。
- **验证：** 57 个开发包通过，新增代码 lint 为 0；完整 lint 的 92 项既有 main 告警未伪装成通过。新回归覆盖实际 parser / command / fake API / 子进程等对应行为；公开提交不含真实租户、凭据或生产私有日志。
- **证据：** [补丁和验证记录](../../evidence/upstream-d2-20261029/m68/README.md)。
- **当前状态：** 提交许可已消除，不再重复询问。仍 blocked 等 upstream 审阅、合入、发布；之后由 worker 审阅 Bex pin，按单独授权发布 CLI，再做本项原 native/shared-surface/清理验收。不是已修复交付，不是 done。
- **范围：** 必要的 commit/push 均在独立 puncsky/cli fork 分支；本轮没有提交或 push bex main，也未触碰其他会话的 source 工作。
