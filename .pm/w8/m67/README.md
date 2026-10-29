# w8 · m67 — Keep native live logs running after a transient disconnect

**Worker:** worker8 **Goal:** the released Bex CLI follows logs through transient connection loss with visible retry status and exact record resumption. **Status:** todo（2026-10-29 解除阻塞；原记录：t001 done (v2.29 candidate qualified; the reconnect fix works, but its retry of normal closes conflicts with Bex's terminal build-tail close); t002–t007 wait on a user decision about how a tail ends, then a requested CLI release

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Qualify the released reconnecting tail against Bex lifecycle controls](t001.md) — **DONE** | 45m | — |
| t002 | [Adopt the qualified upstream pin and review its hosting wire delta](t002.md) — **BLOCKED (build-tail end-of-stream decision)** | 35m | t001 |
| t003 | [Verify affected hosting callers through candidate Bex and upstream CLI](t003.md) | 30m | t002 |
| t004 | [Render parity](t004.md) | 20m | t003 |
| t005 | [Simplify](t005.md) | 10m | t004 |
| t006 | [Test coverage](t006.md) | 20m | t004 |
| t007 | [Closeout](t007.md) | 10m | t005, t006 |

Total: 2h50m, seven tasks. [Researched finding](finding.md), [native fault/recovery evidence](evidence/cli-sweep272/README.md).

## Definition of done

- The exact adopted upstream pin is recorded and Bex's embedded consumer passes the transient-disconnect controls: visible status, resumed records A/B exactly once, continued follow until explicit cancellation.
- Changed supported hosting wire/output contracts, Bex branding/config/device-login seams and completed/no-build tails are qualified. Existing YAML/TUI limitations are honestly graded; unrelated commands retain their supported/partial/non-goal classification.
- Required CLI/pin checks and parity acceptance pass with all own fixtures/configs/processes cleaned.
- After an explicitly requested CLI release, published Bex delivery embeds the qualified fix and native acceptance passes. Candidate-only qualification leaves this delivery gate open; filing does not authorize a release. Current published0.3.2 remains affected until that happens.
- All status/frontmatter/paths are synchronized and the milestone moves to done only after these observable gates hold.

## Source + Goal linkage

- **Source:** functional `$qa-find-bugs-cli` w8/sweep272. Six current-pin failure cases and six current-pin recovery failures silently exit0; the released upstream2.29 tag build passes three recovery controls. Upstream has already implemented the fix; this is Bex dependency adoption, not another upstream implementation request.
- **Goal linkage:** ADR008 usable hosting and ADR018 accurate Render CLI compatibility; a successful log follow must continue observing a service when an ordinary connection drops.
- **Expected outcome:** Bex's customer CLI visibly retries transient loss, resumes without replay duplicates and exposes terminal failures honestly.
- **Why now:** the current Bex0.3.2/Render2.27 consumer loses the follow silently; upstream2.29.0 was published2026-10-12 and provides a tested fix. Qualify its new retry/normal-close policy before broad pin adoption and release. This spans dependency, shared hosting contracts and delivery, so it is a multi-task milestone.
- **Render parity included:** the imported CLI is the user-facing surface, and its generated contracts plus existing REST/GraphQL/MCP/dashboard semantics need explicit review. No backend defect is asserted by the loopback reproduction.

## Boundaries

Current pin remains2.27/a764810a7682; it is the contract oracle until qualified adoption. The new2.29 comparison is an unmodified tag build, not a Bex candidate or installed release. No production failure was induced. The hunt changed only PM evidence, with no dependency edit, commit/push, external submission or release. See finding.md for source locations, controls, dedupe and unverified areas.

## Qualified, parked on a lifecycle decision — 2026-10-14 (`/loopx w8`)

The v2.29 candidate fixes silent termination (see [evidence](evidence/candidate-2.29-20261014/README.md)), but it retries 1000 closes. Bex's w8/020 terminal build-tail refusal closes with 1000, so adoption needs the user to choose:

- **(A)** Send a non-retryable close code for named refusals; v2.29 then exits non-zero with the reason.
- **(B)** Accept indefinite retry for finished build tails.
- **(C)** Wait for an upstream end-of-stream signal.

After that, t002–t006 can proceed. t007 also needs an explicitly requested CLI release. No pin change is committed.


## 解除阻塞记录（2026-10-29，`/unblock w8`）

- **涉及 ID：** w8/m67
- **结论来源：** 用户明确决定（D2）。
- **结论：** 「没有正在运行的 build」时，服务端改用**不可重试的关闭码**（非 1000）结束 `logs --type build --tail`，CLI 以非零退出码结束并显示原因；然后升级到 v2.29 的重连行为。不采用「等上游修复」。
- **剩余门槛：** 实施完成后，用户面的最终交付依赖一次 CLI 发布（`bex-cli/v*`，由用户发起 `/release cli`）。这只是收尾门槛，不阻塞 t002–t006 的实施。
- **worker 下一步：** loopx 实施 t002–t006：选定关闭码，服务端加测试，CLI 在 v2.29 上验证恰好一次重连 A/B 且无运行中 build 时不循环。实施完成后如只剩 CLI 发布，再回到 blocked 并写明等用户发布。
