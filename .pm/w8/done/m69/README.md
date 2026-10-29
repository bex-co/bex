# w8 · m69 — Preserve native cron startup semantics

**Worker:** worker8 **Goal:** a native cron runs its accepted startup command through the native launch path and reads it back correctly. **Status:** done (2026-10-29)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Keep native cron start commands in the native launch role — **DONE** | 45m | — |
| t002 | Keep readback and shared command roles coherent — **DONE** | 45m | t001 |
| t003 | Render parity — **DONE 2026-10-29** | 25m | t002 |
| t004 | Simplify — **DONE** | 10m | t003 |
| t005 | Test coverage — **DONE** | 30m | t003 |
| t006 | Closeout — **DONE 2026-10-29** | 15m | t004, t005 |

## Definition of done

- Fresh native Bex and unchanged Render create a Node cron on an explicitly approved paid plan with an inline finite Bash command; deployment is Live, a finite API-only manual run is successful, and native logs contain the exact marker without a Bash wrapper workaround. Repeat native command updates and read back the correct command.
- Initial/create/update/Blueprint/clone/generate and shared REST/GraphQL/MCP/dashboard roles stay coherent. Native baked startup remains distinct from explicit image overrides. Opaque Docker/image aliases, clearing, legacy fallback and no-op behavior remain correct, as do carrying-release/source switches.
- Producer/consumer/readback regressions and affected required checks pass. Six native runtimes and buildpack/opaque siblings have honest source/runtime coverage. The exact deployed fix and all remaining limits are identified; no claim of Render-production parity from a local fixture alone.
- Every owned parent/deploy/run/container/session created for acceptance is cleaned and baseline/quota reconciled. Complete all tasks before moving the milestone and tasks to done with synchronized statuses and no stubs.

## Source + Goal linkage

- **Source:** functional `$qa-find-bugs-cli` w8/sweep285; [finding](finding.md), [durable live/native evidence](evidence/cli-sweep285/README.md).
- **Goal linkage:** ADR008 usable hosting, ADR018 Render compatibility, ADR004 native launch and ADR049 declarative command fidelity; typed native StartCommand contract.
- **Expected outcome:** accepted native Bash commands run successfully, and readback/clone retains their real native start text.
- **Why now:** both actual native clients create Live Free services whose runs fail in /bin/sh, and the complete decoder→typed App→operator→container chain proves override promotion. A naive initial routing fix then breaks native readback, also proved, so the consumer correction belongs in the same milestone.
- **Sizing:**170m / six tasks across command routing, shared consumers, parity and hosted acceptance; exceeds an inbox note.
- **Render parity:** included for tenant-facing CLI/API/UI behavior. The client wire is already valid; Bex owns the correction. Independent Render production is unverified.

Severity major. The two private proposal fragments prove only initial routing and native projection, not a complete fix. Two Free parents/two deploys/two runs and four local containers are cleaned; baseline6/quota6/terminating0. No product edit, dependency change, upstream submission, deployment, commit or push by QA.

## Independent mutation-transition replay — sweep286

[Actual local REST and Blueprint evidence](evidence/cli-sweep286/README.md) confirms both native update handlers return200 but put the new native program in Command while leaving the old baked StartCommand. A genuine valid Blueprint command edit does the same and opens one deploy; initial and JSON-reloaded unchanged controls retain the correct role. These are actual producer/storage proofs of the previously source-established siblings, not hosted update acceptance. The excluded first Blueprint fixture lacked YAML quoting; the corrected run reaches the real mutation. Keep this within m69; no duplicate finding or product fix.

## Fresh live initial-role acceptance — sweep302

[Two-client current-source/live evidence](evidence/cli-sweep302/README.md) records24 focused named source passes across10 existing tests. Fresh published Bex and unchanged Render each create Live Free Node cron fixtures, but their one finite API-only manual run each finishes unsuccessful; both native logs return `/bin/sh: 1: [[: not found` and no success marker. Exact deployed revision is unobserved, so this extends the original acceptance failure without filing a duplicate or claiming a source regression. Native update/readback/clone and dashboard acceptance remain unexecuted after the initial failure. Two parents/two deploys/two runs are fully cleaned, baseline6/quota6/terminating0; no product changes or deployment by QA. Keep t003/t006 and the milestone blocked.

## Paid-only cron policy after sweep302

Main bed5e2c32 implements [w8/048](../048.md): new cron jobs require a paid plan. Sweep302 and the original Free fixtures are historical results from before this policy change; they do not authorize another Free create, reuse of a baseline cron, or a paid resource. The hosted startup/readback/clone acceptance now needs an explicitly approved paid fixture in addition to the deployed fix. This continuing free-only hunt skips new cron mutations. Keep t003/t006 blocked; do not close them on source tests or treat a new Free-plan refusal as this startup bug. Existing Free crons are left alone.


## 解除阻塞记录（2026-10-29，`/unblock w8`）

- **涉及 ID：** w8/m69
- **结论来源：** 用户明确决定（D4）+ 本次复核证据（生产 pin `b33dd43c4428` 已含修复）。
- **结论：** 允许 worker 在自有 `bex-canary` workspace 中建**短时付费 fixture**（starter 档，只建验收必需的最小数量），验收后立即按 ID 删除并留 404 证据。
- **用户门槛：** 无。
- **worker 下一步：** loopx 执行 t003/t006：在 starter fixture 上用两种客户端创建 native cron，手动跑一次有限运行并检查标记输出，验证 update/readback/clone，然后清理。上次两次 live 运行仍在 `/bin/sh` 失败（见上方 Sweep302），如仍复现先修复。

## Closeout — 2026-10-29 (`/loopx w8`)

Hosted acceptance on production (pin `b33dd43c4428`), workspace bex-canary, human device-login QA grant. Two Starter cron fixtures under the user's D4 paid-fixture decision, each with the finding's inline Bash command (`[[ 1 = 1 ]] && printf '%s\n' <marker>`), Node runtime, `examples/runtime-version/node`, build `/bin/true`:

| Client | Fixture | Create | Run 1 | Native update → run 2 |
| --- | --- | --- | --- | --- |
| Released Bex v0.3.3 (no `--plan`) | `srv-dbhgk1llp6pc7389fmig` | exit 0, `plan: starter`, `startCommand` = the Bash text, `dockerCommand` null; `dep-dbhgk1llp6pc7389fmj0` Live | `crr-vgu8l26acbsh11m1bqrv` successful, log `LX8C-bx-ONE` | `--cron-command "[[ 2 = 2 ]] …TWO"` exit 0; readback `startCommand` updated; `crr-a7a1ibine0c22mjom3ch` successful, log `LX8C-bx-TWO` |
| Pinned Render v2.27.0 against Bex (`--plan starter`) | `srv-dbhgk3dlp6pc7389fml0` | exit 0, same roles; `dep-dbhgk3dlp6pc7389fmlg` Live | `crr-jsr6tca5puu7sq02kt43` successful, log `LX8C-rx-ONE` | exit 0; readback updated; `crr-s593p4nljtl1fjhf51q8` successful, log `LX8C-rx-TWO` |

No run logged `/bin/sh: 1: [[: not found` (the sweep302 failure). Clone/readback: `POST /v1/blueprints/generate` for the Bex fixture emits the native command as `startCommand` (not `dockerCommand`) with `runtime: node`, `type: cron`, `plan: starter`. Explicit image overrides, opaque aliases, legacy fallback and the GraphQL/MCP/dashboard roles are covered by the t005 regressions, not re-run live. Cleanup: both deleted by ID; `GET /v1/services/<id>` and `/v1/cron-jobs/<id>/runs` → 404.
