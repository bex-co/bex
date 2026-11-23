# w8 · m67 — Keep native live logs running after a transient disconnect

**Worker:** worker8 **Goal:** the released Bex CLI follows logs through transient connection loss with visible retry status and exact record resumption. **Status:** done (2026-11-23)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Qualify the released reconnecting tail against Bex lifecycle controls](done/t001.md) — **DONE** | 45m | — |
| t002 | [Adopt the qualified upstream pin and review its hosting wire delta](done/t002.md) — **DONE 2026-10-29** | 35m | t001 |
| t003 | [Verify affected hosting callers through candidate Bex and upstream CLI](done/t003.md) — **DONE 2026-10-29** | 30m | t002 |
| t004 | [Render parity](done/t004.md) — **DONE 2026-10-29** | 20m | t003 |
| t005 | [Simplify](done/t005.md) — **DONE 2026-10-29** | 10m | t004 |
| t006 | [Test coverage](done/t006.md) — **DONE 2026-10-29** | 20m | t004 |
| t007 | [Closeout](done/t007.md) — **DONE 2026-11-23** | 10m | t005, t006 |

Total: 2h50m, seven tasks. [Researched finding](finding.md), native fault/recovery evidence (`evidence/cli-sweep272/README.md`, purged 2026-11-04).

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

The v2.29 candidate fixes silent termination (`evidence/candidate-2.29-20261014/README.md`, purged 2026-11-04), but it retries 1000 closes. Bex's w8/020 terminal build-tail refusal closes with 1000, so adoption needs the user to choose:

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

## Implementation — 2026-10-29 (`/loopx w8`, decision D2)

- **Server (D2, option A):** `lego/backend/internal/logs/rest.go` ends a WebSocket tail it will not continue (no running build, logs unavailable) with the `==>` refusal line and then close **4000** (`CloseTailRefused`) carrying the reason, instead of 1000. Render CLI v2.29 treats 4000 as terminal and exits non-zero with the reason; v2.27 clients end on any close as before. Tests: `TestWebSocketBuildTailSendsTheRefusalAsALogLine` (close code + reason), `TestCloseReasonFitsTheFrame`.
- **Pin (t002):** `render-oss/cli` → v2.29.0 `af9bdd180e90ab9d1e971d73185687fed9ef08d8` (module `v1.1.3-0.20261012175920-af9bdd180e90`, Go 1.27.2; `lego/go.work` follows). `UPSTREAM_RENDER_CLI.md`, `scripts/bex-cli-validate.sh`, `scripts/bex-cli-build.sh` (embedded upstream version 2.29.0) and the docs that name the pin are updated. Go 1.27.2 needs golangci-lint v2.15.0 (v2.13.2 cannot read 1.27.2 export data), so the Makefile and `.custom-gcl.yml` move to v2.15.0; its new errcheck finding in `cmd/native-init/main_test.go` is fixed.
- **Wire delta review (t002/t004):** v2.29 `ea sandboxes create --network-policy` sends `networkPolicy.type` (Render renamed `default`, now an alias). bex-api accepted only `default` under strict decoding, so every v2.29 create with a policy would have failed `unknown field "type"`. REST/GraphQL/MCP now accept either, answer both, refuse `rules` and a disagreeing pair by name, and refuse `--from <name>` (`snapshotName`) like `--snapshot-id`. Root help's new "Start here (for AI agents)" block named `render skills install` and Render's skills; the branding overlay now prints `bex skills install --output json`. Full grading is the round-5 bullet in [`docs/cli-compatibility-checklist.md`](../../../docs/cli-compatibility-checklist.md).
- **Tests (t006):** `lego/cli/logs_tail_contract_test.go` drives the real binary against a loopback API: a drop after A resumes with A and B exactly once, visible retry status and the process still following; refusal + close 4000 makes one connection and exits non-zero with the reason on stderr. Both fail at the v2.27.0 pin. `cd lego/cli && go test ./...`, `bash scripts/bex-cli-validate.sh`, `make lint` (all four modules), backend `internal/{api,logs,sandbox}` pass.
- **Remaining:** t003 — after this deploy, run the candidate binary against production: reconnect continuity on a live tail, a finished-build tail exiting non-zero once, and `--network-policy deny-all`. t007 — needs a `bex-cli/v*` release (user-run `/release cli`) and the same check on the published binary.

## Hosted candidate check (t003) — 2026-10-29 (`/loopx w8`)

Production pin `03043c7df363` (carries the close-4000 change). Candidate `bex` built from `82228a3a4` (Render CLI v2.29.0). Fixture: free busybox service `srv-dbhj78cjakpc739ce5cg` logging `LX9P-TICK <boot> <n>` every 2 s, deleted afterwards (404).

- **Live tail through interruptions:** across a service restart one process received 199 lines over 150 s with no duplicates or gaps; across a real dropped connection (client paused 120 s) it printed `Unable to connect to logs. Retrying in 1s.` then `Log connection restored.` and delivered the paused-window lines exactly once (193 lines, no duplicates or gaps).
- **Build tail with no running build:** the candidate prints `==> no running build is available to follow` once and `Error: websocket: close 4000: …`, exits 1 after 1.2 s, one connection. Released v0.3.3 prints the same line and exits 0.
- **`--network-policy deny-all`:** accepted live — the sandboxes came back with `networkPolicy {type: deny-all, default: deny-all}`. (Three short-lived sandboxes were created unintentionally while trying to capture the request — a `BEX_HOST` override lost to the config's `api.host`; each ran about a minute and was stopped, `GET` → 404.) Create-nothing REST probes: a disagreeing `type`/`default` pair → `SANDBOX_NETWORK_POLICY_INVALID`; `rules` refused by name.

t003 is done; t007 needs the next `bex-cli/v*` release (user-run `/release cli`) and the same three checks on the published binary.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m67。
- **结论来源：** 2026-10-29 GitHub release / 当前源码复核。Bex 最新发布仍是 2026-10-16 的 `bex-cli/v0.3.3`，早于本项 10-28/29 完成的 launcher / pin 工作。
- **处理：** 原本地/候选验收保留，继续 blocked 等用户明确 `$release cli`，再按本项原 published-binary DoD 验收并清理。`$unblock` 不授权发 tag / 发布；本次也未发布。与 090 不同，v0.3.3 不含这里的新变更。
- **worker 下一步：** 发布实际完成后才恢复最终任务；不能用 main 或 candidate 的成功关闭最终交付门槛。

## Unblock & published-binary verification (t007) — 2026-11-23 (`/loopx w8`)

- **Affected ID:** `w8/m67` (`w8/m67/t007`).
- **Source / gate cleared:** verified GitHub release `bex-cli/v0.9.0` (`06a28e1159`, published 2026-11-20; supersedes the 2026-10-29 `v0.3.3` baseline) and production pin `1a5a8e872398` (`e6c65f5c62`). Both carry `b321637c16` (Render CLI v2.29.0 reconnect + close-4000 refusal) and `c9f3f970f1` (`networkPolicy.type`/`default` resolver); `bex-cli/v0.9.0` embeds Render CLI `v2.36.0` (`4fd34231b637`).
- **Distribution check:** `BEX_VERSION=0.9.0 sh scripts/install-bex.sh` (`linux/arm64`, isolated temp dir) verified the Sigstore cosign signature on `checksums.txt` (`Verified OK`) and the archive SHA-256; `bex -v` printed `bex v0.9.0` / `compatible with Render CLI v2.36.0`.
- **Published-binary loopback controls:**
  - Abrupt TCP drop after `LINE-A` (reconnect replays `LINE-A` then `LINE-B`): 2 connections, `LINE-A` and `LINE-B` delivered once each, stderr `Unable to connect to logs. Retrying in 1s.` → `Log connection restored.`, process stayed alive until `SIGINT`.
  - Close `1011` after `LINE-A` (same overlap replay): 2 connections, `LINE-A` and `LINE-B` delivered once each, same visible retry/restored status, alive until `SIGINT`.
  - Terminal refusal (`==>` line + close `4000`): 1 connection (no retry), refusal printed once on stdout, `Error: websocket: close 4000: …` on stderr, exit `1`.
  - Root `--help`: retains `bex skills install` branding with no `render skills install` leak.
- **Published-binary live production acceptance (`bex-canary` `tea-daif693dqjvc73e7as3g`, isolated device OAuth login, `cli.yaml` `0600`):**
  - **Live tail through a real dropped connection:** disposable free busybox `web_service` `srv-dc215i7qbg1c739j1i30` (`qa-20261123-467a2f69-m67`, live in 30s) tailed via `bex logs -r srv-dc215i7qbg1c739j1i30 --tail --confirm -o text`. Across a 130s `SIGSTOP` pause inducing a real WebSocket drop, the process printed `Unable to connect to logs. Retrying in 1s.` then `Log connection restored.`, delivered 80/80 ticks (`17..96`) with 0 duplicates and 0 gaps, and stayed alive until explicit `SIGINT`.
  - **Build tail with no running build:** `bex logs -r srv-dc215i7qbg1c739j1i30 --type build --tail --confirm -o text` (8s bounded) stayed open quietly waiting for the next build with empty stdout/stderr, matching current post-`w8/m159` production semantics (tails with no log source still end with close 4000, verified by the loopback control above).
  - **`--network-policy deny-all`:** `bex sandboxes create --name qa-20261123-…-m67 --network-policy deny-all --confirm -o json` returned `sbx-dc215a8oodks73auddt0` with `networkPolicy: {default: deny-all, type: deny-all}`, then stopped (`status=terminated`). Negative create-nothing REST probe with disagreeing `type`/`default` returned HTTP 400 naming the conflict.
- **Cleanup:** `DELETE /v1/services/srv-dc215i7qbg1c739j1i30` → 204, `GET` → 404, workspace baseline preserved (`3/3` services); `bex logout` removed isolated `cli.yaml`; temporary Kratos QA sessions revoked via `scripts/qa-login.sh --logout`; temporary install directory removed.
- **Disposition:** all DoD criteria hold on the published `bex-cli/v0.9.0` binary; `t007` marked `done` and `w8/m67` moved to `w8/done/m67/`.
