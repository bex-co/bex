# w8 · m57 — Preserve literal string lists in interactive CLI forms

**Worker:** worker8 **Goal:** applying unchanged log filters preserves their literal terms and selected result set. **Status:** blocked — [upstream PR #72](https://github.com/render-oss/cli/pull/72) 已提交；等 upstream merge/release、Bex delivery 和原线上验收

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Prepare an upstream lossless string-list form correction — **DONE 2026-10-12** | 40m | — |
| t002 | Verify shared form callers and neighboring value types — **DONE 2026-10-12** | 20m | t001 |
| t003 | Adopt the accepted upstream correction through the CLI pin — **BLOCKED: PR #72 submitted; upstream merge/release + Bex delivery + original acceptance** | 30m | t002 |
| t004 | Render parity | 20m | t003 |
| t005 | Simplify | 10m | t004 |
| t006 | Test coverage | 25m | t004 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

- On a fresh owned Free fixture emitting `LEFT,RIGHT`, `LEFT only`, and `RIGHT only`, native Bex and the adopted unmodified Render client use whole-second start/end bounds. `--text '"LEFT,RIGHT"'` initially returns only the combined row. Opening `/` and applying unchanged filters retains one literal `text` query term and that same row; editing an unrelated filter also preserves the term.
- Explicit `--text LEFT,RIGHT` continues to send two OR terms and return all three rows. Preserve ordinary multi-term values, embedded quotes, Unicode, empty/default distinctions where accepted, and CRLF bytes. A CSV reader that removes carriage returns is not a complete correction.
- On a fresh validly created service renamed to a comma-containing display name, native quoted `--resources` resolves its exact owned ID before and after unchanged Apply. Retain the complete name; do not look up its fragments or report the trailing fragment missing.
- Verify the counted shared callers and relevant resource selectors. Preserve enum multi-select and numeric-list conversion, and do not rewrite server log search, resource IDs, wire schemas or dashboard filters to compensate for a client defect.
- The correction is upstream, accepted and included in a reviewed CLI pin/release. A prepared patch alone is not Done. External submission and release need their own authorization; until those dependencies are satisfied, retain this milestone's evidence and park dependent work accurately.
- The failing journey, controls and shared-call tests pass; every fresh live fixture is deleted with detail/list/runtime absence and quota convergence. Complete standing tasks and move the milestone only when the observable behavior holds.

## Source + Goal linkage

- **Source:** user-requested continuous functional `$qa-find-bugs-cli`, w8, 2026-10-11, sweeps88–89. [Finding](finding.md) and [native wire evidence](evidence/native-ui.json) preserve both client reproductions and complete log responses.
- **Goal linkage:** ADR008 reliable hosting, ADR010 precise log inspection, ADR018 and the imported Render CLI contract. A filter must retain the literal values the CLI already accepted.
- **Expected outcome:** Apply preserves selected strings rather than silently widening one comma-containing term into two OR terms.
- **Why now:** both released Bex and the exact pinned upstream binary reproduce this on a healthy owned service; four normal noninteractive controls prove the server handles both query shapes correctly.
- **Render parity included:** the user-facing CLI changes; verify the actual upstream contract and unchanged REST/GraphQL/MCP/dashboard semantics. No Render production account was tested.
- **Scope:** researched filing only, seven tasks / 2h35m. No Bex fork/vendor patch, backend workaround, external message, product fix, commit or push is authorized by this QA filing.

## Progress / block

- **2026-10-12:** prepared `evidence/upstream-lossless-string-list.patch` against the pinned upstream module and `evidence/upstream-prep-audit.md`; `go test ./pkg/command ./pkg/tui/views` passed in a temporary upstream checkout. Tasks t001 and t002 are done.
- **Blocked:** t003–t007 require the user to authorize external submission to `render-oss/cli` after a fresh dedupe, then upstream acceptance/release, a Bex CLI pin/release, and native Bex/Render receipts. No Bex fork/vendor patch or backend workaround was applied, honoring the milestone scope and DO_NOT_DO no-fork rule.

## Unverified

Live text and a comma-containing owned service display-name selector were exercised in both native clients. Path filters and workflow environment forms share the conversion mechanism but were not live tested. The existing offline CRLF control passes before any correction. The external upstream issue index is not exhaustively searched; dedupe again before separately authorized submission.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m57；D2 上游提交批次共 19 项，完整映射见 workstream README 本次复核表。
- **结论来源：** 本次只读 PR 去重和原已准备补丁记录。2026-10-29 `render-oss/cli` 最近 100 条全状态 PR 返回 28 条；没有发现本批补丁的对应提交，#54–#57 仍 open。PR 索引不是最新源码已修复/未修复的充分证据。 本次还对全部 19 份补丁执行 `git apply --check`，均可应用于当前 pin `af9bdd180e90`（Render v2.29.0）；这不是最新 upstream main 已审核或完整测试通过的声明。
- **剩余门槛：** D2 待用户明确允许以其 GitHub 身份公开提交这些已准备的最小补丁；本次没有对外消息、PR、push 或跟进。提交前由 worker 逐项重查 upstream main / issue、刷新与重测补丁；已被上游修复的应按证据收敛，不重复提交。
- **后续：** 即使允许提交，也仍需 upstream 接受/发布、Bex 审阅 pin 更新及单独授权 CLI release，之后逐项执行原 native / shared-surface / cleanup 验收。不 fork、不修请求来掩盖客户端缺陷。本项继续 blocked，不把允许提交当成已合并或已交付。

## 用户批准 D2（2026-10-29，提交对 Render CLI 有帮助的修复）

- **来源：** 用户明确回复「D2 approve to submit changes that are helpful」。此许可适用于已列出的 19 项，提交确实仍对上游有帮助的修复，不是无条件制造 19 个 PR。
- **授权范围：** 在独立 upstream checkout 审核最新 main、去重、刷新补丁、补行为测试；必要的 fork 分支 commit/push 和向 render-oss/cli 提交 PR 是本次明确授权提交的组成部分。不修改/发布 bex 本仓库代码，不给 bex main 新增 commit/push 权限，不重复询问已给出的提交许可。
- **判定：** 已修复或重复的附证据跳过；可证实通用缺陷且验证充分的提交最小修复；没有通用依据的保留说明，不把 bex 特有差异强推上游。公开内容只含通用复现、修复及测试，不带租户、凭据或生产私有日志。
- **剩余：** 每项记录独立的 PR 链接、上游基线、测试/限制；提交不等于合入或发布，原 adoption / native / cleanup DoD 保留，仍受真实 upstream merge/release 门槛约束。

## D2 已提交上游（2026-10-29，PR #72）

- **结论来源：** 用户明确批准 D2 + 最新 upstream 源码复核、行为测试和本次实际提交。
- **涉及 ID：** w8/m57。[Preserve literal string-list elements through TUI forms](https://github.com/render-oss/cli/pull/72)，head `fd2aa3cad96f291768887c43e6a2510e98b23876`，base `c31e586eb6f38138af3cc986b079e46a78c61de7`。
- **验证：** 57 个开发包通过，新增代码 lint 为 0；完整 lint 的 92 项既有 main 告警未伪装成通过。新回归覆盖实际 parser / command / fake API / 子进程等对应行为；公开提交不含真实租户、凭据或生产私有日志。
- **证据：** [补丁和验证记录](../../evidence/upstream-d2-20261029/m57/README.md)。
- **当前状态：** 提交许可已消除，不再重复询问。仍 blocked 等 upstream 审阅、合入、发布；之后由 worker 审阅 Bex pin，按单独授权发布 CLI，再做本项原 native/shared-surface/清理验收。不是已修复交付，不是 done。
- **范围：** 必要的 commit/push 均在独立 puncsky/cli fork 分支；本轮没有提交或 push bex main，也未触碰其他会话的 source 工作。
