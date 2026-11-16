# w22 · m413 — A Stop turn keeps its work: the interrupted turn's disk is retained and the next message continues from it, as on Devin

**Worker:** worker22 **Goal:** after a user stops a turn, the agent's uncommitted and unpushed work is still in the checkout when the next message starts, for a bounded time and with the user told how long, and Stop shows within seconds. **Status:** todo (t001–t007 done; t008's live check is unblocked 2026-11-16 by the user's decision D4, see "解阻记录 2026-11-16"; t009 closes after it)

## Tasks (in order)

| id | title | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------- | ----- | ------------------ |
| t001 | Stop shows within seconds: the Completer reads a stop-requested session's status on a short tick — **DONE** | 45m | — |
| t002 | Retain: an interrupted turn's disk is kept as an image, counted by the storage cap, bounded in time and deleted when it ends — **DONE** | 3h | t001 |
| t003 | Continue: the next message boots a fresh sandbox from the retained image, and falls back to a fresh one when it is gone — **DONE** | 2h30m | t002 |
| t004 | Show retention: the dashboard, API and CLI say how long the changes are kept, and that they were lost if they expired — **DONE** | 1h30m | t002 |
| t005 | Render parity (Devin) — **DONE** | 30m | t003, t004 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005 |
| t008 | Live check: an interrupted turn's uncommitted work is there in the next turn | 1h | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

- A turn that is stopped while it has uncommitted changes and an unpushed commit leaves both in the checkout of the next message's turn, on REST, GraphQL, MCP, the dashboard and `bex agent`, for the retention time (default 24 h, set by a named setting, always shorter than Devin's 14 days)
- The retained image counts against the workspace storage cap, is deleted on expiry, cancel, delete, archive, workspace purge and after a successful continuation, and a full cap or a failed pause degrades to today's behavior with the loss shown
- A Stop turn settles and shows within about 5 s
- The driver still never commits or pushes (ADR090 is unchanged); ADR047 D12 and D6 and ADR018 record the design and the corrected bound

## Source + Goal linkage

- **Source:** inbox note w22/012 (promoted 2026-11-14 by `/loopx w22`), user decision D3 of `/unblock w22` on 2026-11-14 (option A: keep the interrupted turn's work by retaining the sandbox's disk, no change to ADR090's Git custody), and the sandbox lifecycle map of the same day, which showed that a literal OpenSandbox resume keeps the old prompt, secrets and stale status files, so the worker chose to retain the disk as an image and boot the next turn's sandbox from it. Devin: a sleeping session's machine is kept and can be resumed for 14 days ([release notes](https://docs.devin.ai/release-notes/2026), 2026-09-25, 2026-10-05, 2026-10-09).
- **Goal linkage:** ADR008's AI-native thesis and pillar 5 (agent sessions); the reference product is Devin ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md)).
- **Expected outcome:** a customer who stops an agent mid-task and says "continue" keeps the half-written code, instead of finding it gone.
- **Why now:** w22/m403 shipped Stop turn on 2026-11-14 with this as its largest known gap, and the user chose the fix the same day.
- **Render parity included (reference product: Devin):** REST, GraphQL, MCP, the dashboard and the CLI change.
- **Deviation from the decision's wording, stated:** the decision said "suspend the sandbox and resume it". The mechanism here is "retain the disk as an image and start the next sandbox from it", with the same outcome for the customer (the work survives, ADR090 untouched, bounded retention, absorbed cost), because a resumed pod would keep the old turn's env and files.

## Gate: t008 (the live check) — cleared 2026-11-16, see "解阻记录 2026-11-16"

(The gate as it stood on 2026-11-15, kept for history.) **BLOCKED — needs, from the user or infra, a running stack with a sandbox cluster, or the user's authorization for one production agent probe.** Everything shipped is verified against fakes, real Postgres and the CLI, dashboard and API contract tests; nothing has yet watched a real sandbox keep its disk through a Stop and boot the next turn from it. As of 2026-11-15 the machine this ran on has `docker` (colima, 4 CPUs, 7.7 GiB) but no `kind` or `clusterctl`, so `scripts/dev-env.sh <N> up` with the agent leg cannot bring up the shared kind/CAPD cluster (the same gate as [m403 t011](../m403/README.md) and m402 t006; one environment clears all three).

Two ways to clear it (the same two paths as m403, recorded by `/unblock w22` on 2026-11-14):

1. **A machine that can run `scripts/mock-cluster.sh`** (Docker VM of at least 16 GiB and ghcr.io reachable, then the worker installs `kind` and `clusterctl` itself). Then t008 runs the scenario in its steps with a shortened `BEX_AGENT_INTERRUPT_RETENTION`.
2. **The user authorizes an agent-session probe on production's weekly matrix agent leg** (`BEX_VERIFY_AGENT_DRIVER=1`, w7/m393), beyond the 2026-11-14 D1 authorization (which covers disposable-workspace sandbox probes, not model-spend agent turns). The probe: start a turn that writes an uncommitted file, Stop it, read the turn (`interrupted`, with a retention time), send a second message that reads the file.

Either way the worker's first step is t008 step 1; t009 closes the milestone after it. Run `/unblock w22` for the Chinese walkthrough.

## Progress 2026-11-15 (/loopx w22)

- **t001–t007 shipped:** `b446090a6` (t001), `48c93ceed` (t002, t003), `9fb96b403` and `860321439` (t004), `e8036991b` (the t001/t004 review fixes). ADR018 and ADR047 D12 record the design, the closed divergence (the disk is kept) and the remaining ones (processes are not kept, 24 h by default against Devin's 14 days; a full cap, a failed pause or an unusable image starts fresh and says so on the turn).

## 解阻记录 2026-11-16（/unblock w22）

- **来源：** 用户决定 **D4**（2026-11-16）。`/unblock w22` 里提出的问题推荐项是"本机搭建 + 生产探针"，用户回复 "ok, approve"。下面写的是当时提问里列出的范围；没有点名的不算批准。同一个决定也记在 m402（t006）、m403（t011）和 m312 里。
- **复核证据（同日，只读）：** 这台 Mac 是 Apple M6、12 核、32 GiB，空闲约 80%，load 1.8；数据卷剩 67 GiB（已用 84%），`~/.colima` 已占 18 GB；colima 默认 profile 停止，4 CPU / 8 GiB；`ghcr.io/v2/` 返回 401（可达，之前的 504 是瞬时的）；`kind`、`clusterctl` 没装。m413 的 t001–t004 全部在生产 pin `027e4009e` 里。
- **批准的范围，本机部分：**
  1. 装 `kind`、`clusterctl`（brew，或带 sha256 校验的二进制放进 scratchpad，worker 选）。除此之外不装别的。
  2. colima 默认 profile 改成 8 CPU / 16 GiB 并启动。`bex-w21` 工作树共用这个 Docker，启动前看一眼 `colima list` 和 `docker ps`。
  3. 跑 `bash scripts/mock-cluster.sh` 和 `scripts/dev-env.sh <N> up`、`agent-up`（AGENTS.md 已预批准）。`dev-env.sh` 的 N 只能是 1 到 10，w22 没有自己的槽：先 `status` 选一个没起来的 N，用完 `down`；agent 腿是集群级独占。
  4. 模型只用 `agent-stub` 测试替身（固定回复，不花钱）。它现在不会调用工具、也不会放慢输出，所以 worker 要先扩展它（`scripts/dev-env/agent/model-stub.py`），让一个 turn 能写出文件、能停留到被 Stop；这是代码，单独走 `/ship`，是 worker 的技术判断。**没有批准** `agent-openrouter`（会花 OpenRouter 余额）。
  5. 护栏（worker 的，不是用户说的）：数据卷剩余磁盘低于 25 GiB 就停下，把读数写进新闸门；验证期间不开 Xcode 和 iOS Simulator（内存）。
  6. 结束：`dev-N down`；`docker ps` 没有别的容器时 `colima stop`；保留工具和 16 GiB 配置。
- **批准的范围，生产探针部分（超出 D1，所以单独记）：**
  - 用 `workflow_dispatch` 派发 `isolation-matrix.yml`（环境 `production-observe`，与 D1 同一方式），只用一次性、不计费的 workspace（ADR046），避开每周二 03:41 UTC 的周跑（共用 concurrency）。
  - 一个 agent session，占位 key，把 turn 挂住（现有 agent 腿的做法：没有模型请求离开集群）；在挂住的 turn 上 Stop 一次；在 sandbox 里写一个文件；发一条后续消息，在新 sandbox 里读到这个文件；然后取消 session。
  - 这会在生产 Zot 里多存一个保留镜像。探针必须确认取消之后它被释放（registry 里没有残留），并清理一次性 workspace。
  - **没有批准：** 改保留时间的设置、重启 bex-api、在生产开 2FA 强制、对生产做队列操作（m403）。
  - 这条探针的代码是 `scripts/verify-sandbox-isolation.sh` agent 腿的扩展，单独走 `/ship`，先本地后生产：本地发现的缺陷先修，不把坏探针放到生产。
- **闸门已清除，哪里没清：** 授权已给，机器经核实够用，所以不再有外部闸门。如果 bring-up 失败（ghcr 再次拉不下来、磁盘低于护栏、等待超时），把精确的失败原因写成新闸门再停靠，不要猜。
- **worker 的下一步（顺序）：** 搭环境一次，服务 m402 t006、m403 t011 和本项；然后 t008 的本地部分：步骤 1 用扩展后的替身，步骤 2 把 `BEX_AGENT_INTERRUPT_RETENTION` 缩短后等过期，看到丢失提示、文件没了（生产不能缩短保留时间，所以这一步只在本地）；然后生产探针只验步骤 1；再 t009 收尾。
- **验收：** t008 的两个结果在真实栈上看到（步骤 1 在本地和生产各一次，步骤 2 在本地）；证据只写摘要（`#NO-ARTIFACTS-IN-GIT`）。
