# w8 · m60 — Preserve accepted arguments when copying CLI commands

**Worker:** worker8 **Goal:** Copy from a working CLI log view produces a command that preserves its search and time filters. **Status:** blocked — [upstream PR #73](https://github.com/render-oss/cli/pull/73) 已提交；等 upstream merge/release、Bex delivery 和原线上验收

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Prepare an upstream command-aware Copy formatter — **DONE 2026-10-12** | 50m | — |
| t002 | Verify shared Copy callers and command aliases — **DONE 2026-10-12** | 25m | t001 |
| t003 | Adopt the accepted upstream correction through the CLI pin — **BLOCKED: PR #73 submitted; upstream merge/release + Bex delivery + original acceptance** | 30m | t002 |
| t004 | Render parity | 20m | t003 |
| t005 | Simplify | 10m | t004 |
| t006 | Test coverage | 25m | t004 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

- On fresh owned Free fixtures, actual native Bex and adopted same-pin Render Copy preserve `--text 'LEFT RIGHT'` (one row) and `--text '"LEFT,RIGHT"'` (one literal term, one row). Replay the captured arguments unchanged; each copied result matches the original.
- A working `--text plain --start 30m` view copies an accepted relative time, with exit0 and its expected row. Absolute bounds preserve accepted nanoseconds. No Go struct/pointer representation appears; coordinate the independent082 precision fix.
- Plain and explicit two-OR-term controls retain their results; actual flag-parser checks cover counted shared callers, command aliases, enums, repeated arrays, positional spaces, Unicode, quotes and nil/default/accepted empty neighbors. Preserve bytes accepted by the existing parser, qualifying077's separate CRLF issue.
- The upstream correction is accepted, included in a reviewed Bex pin and released under separate authorization. A locally prepared patch cannot satisfy Done. Preserve existing server/UI filter semantics and the no-fork decision.
- Native evidence records version identities and gaps. Every fresh owned fixture reaches detail/list/runtime absence and quota convergence. Complete standing tasks and perform the final PM move only when all outcomes hold.

## Progress / block

- **2026-10-12:** prepared `evidence/upstream-copy-command-formatter.patch` against the pinned upstream module and `evidence/upstream-prep-audit.md`; `go test ./pkg/command ./cmd` passed in a temporary upstream checkout (13 subtests fail without the fix). Tasks t001 and t002 are done.
- **Blocked:** t003–t007 require the user to authorize external submission to `render-oss/cli` after a fresh dedupe, then upstream acceptance/release, a Bex CLI pin/release, and native Bex/Render receipts. No Bex fork/vendor patch or backend workaround, honoring the milestone scope and DO_NOT_DO no-fork rule.

## Source + Goal linkage

- **Source:** ongoing user-requested functional `$qa-find-bugs-cli` loop, w8, sweep135, 2026-10-11. [Finding](finding.md), [ten native Copy captures](evidence/native-copy.json), [originals](evidence/controls.json), [replays](evidence/replays.json).
- **Goal linkage:** ADR008 dependable hosting, ADR010 precise logs and ADR018's imported Render CLI contract.
- **Expected outcome:** copied commands preserve already accepted search element boundaries and time values.
- **Why now:** released Bex and the unchanged exact upstream client reproduce all three failures against a healthy Free fixture; simple controls pass, and the same serializer remains in upstream main.
- **Render parity included:** a user-facing CLI formatter changes; verify the upstream contract and unchanged REST/GraphQL/MCP/dashboard behavior.
- **Scope:** one shared Copy serialization bug, minor severity, seven tasks /2h50m. Researched filing only; no implementation, upstream submission, release, commit or push in this hunt.

## Unverified

Live coverage is own web-service log phrase/literal-comma/relative-time Copy. Absolute time, Unicode/quote/CRLF neighbors and the parser are actual-pin local probes. Other command copies, datastore tails, unsupported workflows, deployed image revisions and direct Kubernetes teardown are unverified. Forty-two stack sites are a mechanical census, not42 proven affected/live callers. See the finding for setup/ingestion qualifications and completed cleanup.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m60；D2 上游提交批次共 19 项，完整映射见 workstream README 本次复核表。
- **结论来源：** 本次只读 PR 去重和原已准备补丁记录。2026-10-29 `render-oss/cli` 最近 100 条全状态 PR 返回 28 条；没有发现本批补丁的对应提交，#54–#57 仍 open。PR 索引不是最新源码已修复/未修复的充分证据。 本次还对全部 19 份补丁执行 `git apply --check`，均可应用于当前 pin `af9bdd180e90`（Render v2.29.0）；这不是最新 upstream main 已审核或完整测试通过的声明。
- **剩余门槛：** D2 待用户明确允许以其 GitHub 身份公开提交这些已准备的最小补丁；本次没有对外消息、PR、push 或跟进。提交前由 worker 逐项重查 upstream main / issue、刷新与重测补丁；已被上游修复的应按证据收敛，不重复提交。
- **后续：** 即使允许提交，也仍需 upstream 接受/发布、Bex 审阅 pin 更新及单独授权 CLI release，之后逐项执行原 native / shared-surface / cleanup 验收。不 fork、不修请求来掩盖客户端缺陷。本项继续 blocked，不把允许提交当成已合并或已交付。

## 用户批准 D2（2026-10-29，提交对 Render CLI 有帮助的修复）

- **来源：** 用户明确回复「D2 approve to submit changes that are helpful」。此许可适用于已列出的 19 项，提交确实仍对上游有帮助的修复，不是无条件制造 19 个 PR。
- **授权范围：** 在独立 upstream checkout 审核最新 main、去重、刷新补丁、补行为测试；必要的 fork 分支 commit/push 和向 render-oss/cli 提交 PR 是本次明确授权提交的组成部分。不修改/发布 bex 本仓库代码，不给 bex main 新增 commit/push 权限，不重复询问已给出的提交许可。
- **判定：** 已修复或重复的附证据跳过；可证实通用缺陷且验证充分的提交最小修复；没有通用依据的保留说明，不把 bex 特有差异强推上游。公开内容只含通用复现、修复及测试，不带租户、凭据或生产私有日志。
- **剩余：** 每项记录独立的 PR 链接、上游基线、测试/限制；提交不等于合入或发布，原 adoption / native / cleanup DoD 保留，仍受真实 upstream merge/release 门槛约束。

## D2 已提交上游（2026-10-29，PR #73）

- **结论来源：** 用户明确批准 D2 + 最新 upstream 源码复核、行为测试和本次实际提交。
- **涉及 ID：** w8/m60。[Preserve search and time arguments in copied CLI commands](https://github.com/render-oss/cli/pull/73)，head `4734154901377785405c0d969da40769296caa27`，base `c31e586eb6f38138af3cc986b079e46a78c61de7`。
- **验证：** 57 个开发包通过，新增代码 lint 为 0；完整 lint 的 92 项既有 main 告警未伪装成通过。新回归覆盖实际 parser / command / fake API / 子进程等对应行为；公开提交不含真实租户、凭据或生产私有日志。
- **证据：** [补丁和验证记录](../../evidence/upstream-d2-20261029/m60/README.md)。
- **当前状态：** 提交许可已消除，不再重复询问。仍 blocked 等 upstream 审阅、合入、发布；之后由 worker 审阅 Bex pin，按单独授权发布 CLI，再做本项原 native/shared-surface/清理验收。不是已修复交付，不是 done。
- **范围：** 必要的 commit/push 均在独立 puncsky/cli fork 分支；本轮没有提交或 push bex main，也未触碰其他会话的 source 工作。
