# w22 · m403 — A message sent to a running agent session is queued, and Stop ends only the current turn, as on Devin

**Worker:** worker22 **Goal:** a user can send a message to an agent session while its turn is running and it is queued, not refused. Queued messages can be edited, removed, reordered or sent now, and they run in order as the turn settles. A new "Stop turn" ends only the running turn and leaves the session alive and steerable with the reply streamed so far kept; Cancel is unchanged. Dashboard, REST, GraphQL, MCP and `bex agent chat` all use the one server-side queue. **Status:** done (2026-11-16: the live check ran on `dev-10` with the `agent-stub` test double, see "Live check 2026-11-16"; it found and fixed two dashboard defects and two in the test double; codex, gemini, muse, MCP and the CLI were not run live)

## Tasks (in order)

| id | title | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Design record in ADR047 and a driver spike: the queue's rules, the interrupt's semantics, whether the driver and each engine can end only the current turn — **DONE** | 45m | — |
| t002 | The queue: store, service and API: a steer during a turn answers "queued"; list, edit, delete, reorder; REST, GraphQL and MCP — **DONE** | 75m | t001 |
| t003 | The dispatcher: when a turn settles the next queued message runs through the existing steer path; safe across restarts and replicas — **DONE** | 60m | t002 |
| t004 | The interrupt verb: ends only the current turn, the session returns to idle with its reply and branch kept; REST, GraphQL, MCP and the gateway-to-driver path — **DONE** | 60m | t001 |
| t005 | Dashboard: the composer works during a turn, queued-message chips (edit, remove, reorder, send now) and "Stop turn" beside Cancel — **DONE** | 90m | t002, t004 |
| t006 | `bex agent chat`: the server queue replaces the client's one-slot queue — **DONE** | 30m | t002 |
| t007 | Render parity (Devin) — **DONE** | 20m | t003, t004, t005, t006 |
| t008 | Simplify — **DONE** | 20m | t007 |
| t009 | Test coverage — **DONE** | 30m | t007 |
| t011 | Live check on `dev-N`: Stop turn on a real sandbox and the queue and Stop turn walkthrough from the dashboard — **DONE** | 60m | t004, t005 |
| t010 | Closeout — **DONE** | 10m | t008, t009, t011 |

## Definition of done

- Sending a message to a session whose turn is creating, running, resuming or redispatching is accepted and queued on REST, GraphQL and MCP. `AGENT_SESSION_TURN_IN_FLIGHT` is no longer returned for it. A full queue answers a coded 409 `AGENT_SESSION_QUEUE_FULL`, and a canceled or archived session still refuses with its existing codes.
- Queued messages run one per turn in order after the current turn settles (completed, failed or interrupted), each through the existing steer path, so the spend cap, the live-sandbox limit, the branch rules and the prompt-size quota apply at dispatch. A message blocked at dispatch stays queued and shows why. The queue survives a bex-api restart and two replicas never dispatch one message twice (real-Postgres tests).
- A user who can operate the session can list, edit, delete, reorder a queued message and send one now (interrupt the running turn, then run it next). The queue is cleared when the session is canceled, archived or deleted.
- "Stop turn" (REST, GraphQL, MCP and the dashboard) ends only the current turn: the session returns to idle and stays steerable, what the assistant had already streamed stays in the transcript, commits already pushed stay on the branch, and queued messages are kept. Cancel keeps its current meaning (terminal).
- The composer accepts input while a turn runs; queued messages appear as chips; `bex agent chat` sends to the server queue instead of holding one message in the client.
- Every path has an automated test, and a live run on `dev-N` is recorded in this README.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w22` (2026-11-14), filed by `/pmx all to w22`.
  - **bex today:** `lego/backend/internal/agentsessions/service.go` (~lines 1818–1820) answers a steer on a session in `creating`/`running`/`resuming`/`redispatching` with 409 `AGENT_SESSION_TURN_IN_FLIGHT` ("a turn is already running; wait for it to finish before steering"), and the store-level race (`ErrAgentSessionTurnState`, ~line 2126) does the same. `dashboard/src/features/agent-sessions/components/steering-composer.tsx` (~lines 60–76) disables the composer while a turn is live. `bex agent chat` queues one message in the client (ADR106 D2). Cancel is terminal (`AGENT_SESSION_NOT_STEERABLE`).
  - **bex's own design record names the gap:** ADR047 § D9 lists a "mid-run-capable steering composer" among the table stakes held to, and records the public live-turn path as unavailable (`AGENT_SESSION_LIVE_TURN_UNAVAILABLE`).
  - **Reusable parts:** ADR047 § D11 (w9/m344) already queues PR feedback durably per session and delivers it as a steering turn once the session is idle (`lego/backend/internal/agentsessions/pr_feedback.go`: poll, lease, batch; table `agent_session_feedback`, migration 0237). The driver's turn runner takes an external abort signal (`lego/agent-image/driver/src/session.ts`, ~lines 211–226, "agent turn terminated"); whether that reaches a live turn and leaves the session usable is t001's spike.
  - **Devin** ([docs.devin.ai/release-notes/2026](https://docs.devin.ai/release-notes/2026), entries read 2026-11-14): 2026-06-05 "You can now send messages to a session that is waiting for capacity"; 2026-07-17 "You can now set a preference to automatically queue messages while Devin is working"; 2026-08-21 "Press Cmd/Ctrl+Shift+Backspace to stop Devin while it is working"; 2026-08-26 "pressing Cmd/Ctrl+Enter while editing a queued message sends it immediately"; 2026-09-09 "Messages queued in a session are no longer lost when the session goes idle and resumes"; 2026-10-14 interrupting Devin "the part of the reply you already saw stays in the chat instead of disappearing"; 2026-10-16 "Queued messages can be sent, edited, removed or reordered after Devin stops or the session goes to sleep"; 2026-11-06 (Side Chats) "send to interrupt it or queue to run after it finishes".
- **Goal linkage:** ADR008's AI-native thesis and pillar 5 (agent sessions); Devin is the reference product for cloud coding agents (AGENTS.md, ADR047).
- **Expected outcome:** a person can add to or correct a running agent without waiting for the turn to end or killing the session, which today they can only do by cancelling.
- **Why now:** `w9/m344` just shipped the mechanism that starts a new turn on an existing session from an outside event and a durable per-session queue to model on; a user message is a second source for the same loop. The mid-run steering composer is the one item on ADR047's own table-stakes list that is not delivered.
- **Anti-goal check:** `#APPROVAL-MODE` forbids approval gating, human-in-the-loop pauses and an agent-to-user question channel. This is the opposite direction (user to agent), gates nothing and pauses nothing, so it does not conflict.
- **Open recommendations, not user decisions** (t001 records them in ADR047 as recommendations): queue by default with interrupt as an explicit action, because bex has no live injection into a running turn; a per-session cap on pending messages; FIFO one message per turn. Where Devin's default differs (its queueing is a preference, 2026-07-17), t001 records bex's choice in ADR047 and ADR018 as a divergence awaiting the user's sign-off. That sign-off is open and does not block closing the milestone: the repository rule is that parity work is never parked on an optional decision.
- **Render parity included (reference product: Devin):** new fields and verbs on REST, GraphQL and MCP plus the dashboard and CLI, so t007 compares each with Devin.

## Progress 2026-11-14 (/loopx w22)

- **t001 done:** the design is [ADR047 D12](../../../../docs/ADR047-cloud-coding-agent-sessions.md#d12--a-message-to-a-running-session-is-queued-stop-turn-ends-only-the-current-turn-w22m403-2026-11-14), with the Devin entries re-read on 2026-11-14 (two of the dates in the original proposal were paraphrased wrongly and are corrected here and in the ADR: queueing while Devin works is a preference, 2026-07-17; `Cmd/Ctrl+Enter` sends a queued message at once, 2026-08-26). Read-only code mapping found that every steer starts a fresh sandbox and driver, that the driver already has a per-turn `AbortController` wired only to SIGTERM, that there is no bex-api-to-driver push channel (the gateway exec path that reads the status file is the cheapest one), and that no phase constraint needs to change (an interrupted turn finalizes as `completed` with the turn flagged `interrupted`). The spike is by reading; the live check of the abort is t004's.
- **ADR018** records the divergence awaiting the user's sign-off (queue by default, where Devin's queueing is a preference). It does not block this milestone.
- **Shipped (t002–t009):** the queue (`agent_session_queued_messages`, migration 0266; up to 10 pending, FIFO, one per turn, never dropped by age), the dispatcher (a 5 s poll with a lease and `FOR UPDATE SKIP LOCKED`; the transaction that starts the turn also records the message as delivered, so a crash cannot run it twice; D11 feedback yields to pending user messages), Stop turn (`interrupt` on REST, GraphQL and MCP; a sentinel file in the sandbox through the gateway exec path; the driver aborts only that turn and writes `interrupted`; the Completer finalizes it completed with the turn flagged `interrupted`, migration 0267), list, edit, delete and move, the dashboard composer (enabled while a turn runs), queued-message rows with send now, Stop turn beside Cancel, the "interrupted" turn note, and `bex agent chat` on the server queue plus `/stop`. ADR047 D12 has the design and the "As built" notes; ADR018 the Devin comparison.
- **Review findings fixed with tests that fail on the old code:** a message was dropped silently for refusals a person can fix (missing model key, agent unavailable, the 2FA 403, a missing skill or secret): it now stays queued with a reason and retries; the CLI no longer retries `steer` (a lost reply could run a message twice); the driver could abort a turn after it finished; the Go and TypeScript sentinel paths are pinned equal; reading a session no longer lists the whole queue unless asked, and an idle dispatcher costs one query.
- **Checks:** the real-dependency backend suite passed in all 78 packages; backend lint, deadcode, gofmt and `go vet` are clean; driver 227 tests; CLI 12 packages; dashboard `yarn lint` clean and 7491 tests. 17 mutations of the dispatcher, the claim, the Completer branch and the driver were each caught by a test.
- **Open, filed as follow-ups:** an interrupt keeps only what the agent had already pushed and settles within the Completer's 15 s tick ([w22/012](../012.md)); `bex agent stop` and `send --wait` for a queued message ([w22/013](../013.md)); a retried send that starts a turn can run twice, the interrupted status can be forged by an untrusted agent, Move around a leased head, and the retry cadence of a blocked message ([w22/014](../014.md)).

## Live check 2026-11-16 (t011, /loopx w22)

Run on `dev-10` (w22 has no slot) with the agent leg and the `agent-stub` test double, on the local kind/CAPD cluster: bex-api, the SSH gateway, the driver and the Claude Code adapter from this tree, a real OpenSandbox pod per turn, real Kratos and Postgres. No model was called and nothing ran against production. The sessions had no repository (the throwaway GitHub App cannot mint tokens). `dev-10`, its agent leg and its tenant namespaces are removed; the cluster and colima are left running for m413 t008.

**The scenario over REST (session token), as observed:**

1. Two messages sent while turn 1 ran: `202` each, `queuedMessage.position` 1 and 2, `state: pending`. `AGENT_SESSION_TURN_IN_FLIGHT` never appeared in the bex-api log for the whole run. A message sent while the session was `redispatching` was also `202`.
2. `move` of the second to position 1: `200` with the list in the new order. `PATCH` of its prompt: `200`, `updatedAt` moved. `DELETE` of the other: `204`. Editing the deleted one: `404 AGENT_QUEUED_MESSAGE_NOT_FOUND`. A move past the end is clamped.
3. Send now (the message is first, then `interrupt`): `202` in 59 ms. The session read `completed` and steerable 1.4 s later, `redispatching` with turn 2 at 6.3 s and `running` at 8.6 s, with the queue empty.
4. A third message sent during turn 2: `202`, position 1. It ran as turn 3 when turn 2 ended.
5. Stop turn on a running turn with nothing queued: `202` in 140 ms; `completed` and steerable 1.7 s later, and still idle 20 s after that. A second `interrupt`: `409 AGENT_SESSION_NO_TURN_RUNNING`. A follow-up message then answered `200` and started a fresh turn.
6. bex-api restarted with two messages queued behind a running turn: both were still `pending` after the restart. The turn ended, the first ran as turn 6 three seconds later; Stop turn on turn 6 left the second queued, and it ran as turn 7 seven seconds after the Stop.
7. The queue rows afterwards: four delivered rows with `delivered_turn` 2, 3, 6 and 7, one row per turn, none dropped. The transcript lists turns 1 to 7 with each prompt once, in the order above.
8. A second session: ten messages queued, the eleventh `409 AGENT_SESSION_QUEUE_FULL` (`max: 10`). Cancel: the session is `canceled`, the list is empty and the ten rows read `dropped_reason: the session was canceled`; a send and an `interrupt` both answer `409 AGENT_SESSION_NOT_STEERABLE`.

**Stop turn inside the sandbox (claude profile):** `/var/run/bex-agent/` held `status.json` `{"state":"running"}`; the `interrupt` sentinel was there 59 ms after the request and `status.json` read `{"state":"interrupted", …}` by 443 ms (216 ms on the second run), so the driver's 500 ms poll held. The model stub logged both of the engine's open requests dropped ("the client went away mid-stream") within 2 s: the abort reached the engine's process tree. The turn is flagged `interrupted: true` and its transcript keeps what had streamed: 143 one-second lines on the first Stop (the turn had run 143 s), 41 and 22 on the others. Three Stops, 1.4 to 2.5 s to the idle state, against the 15 s read from the code before m413.

**Not run, and why:** codex, gemini and muse (the stub serves only the Anthropic endpoint, and a real key's spend was not approved, decision D4). MCP and `bex agent chat` (covered by their automated tests only; the dashboard walk below exercised GraphQL). A repository session, so no branch and no delivery note for pushed commits. On this stack every interrupted turn read `retention: lost`, `retentionLostReason: save_failed`, and the page said "Your unpushed changes could not be saved and were lost." That is m413's ground (t008 checks it); it is recorded here, not judged.

**The dashboard walk (Chromium, 1440 by 900):** while a turn runs the composer is enabled with "Queue a message. It runs when the current turn ends.", and Stop turn sits beside Cancel. Three messages sent from it appear as "3 queued messages" with positions and Send now, Move up, Move down, Edit and Remove on each. Move up, Remove, and Edit with Save changed the list at once. Send now on the second row put it first, stopped the turn ("Stopping the current turn. This message runs next.") and it ran next; the other followed. The running turn's reply streamed live, and Stop turn returned the page to idle in 3 s with "Turn 10 was interrupted before it finished." and the reply kept. In all, 14 turns on the session, five of them interrupted.

**Defects the check found, each fixed with a test that fails without the fix:**

- **No turn ever finished on `agent-stub`.** The stub streamed with keep-alive and no length, so the reply's body never ended and the engine waited after `message_stop`; the session stayed `running`. The stub now closes the stream. This is the test double, not the product.
- **The stub could not hold a turn.** It now holds one on `stub-hold:<seconds>` in the newest prompt, one `tick <n>` a second, and ignores the conversation the driver replays in `<session-context>` (found when "third queued" was held by an earlier prompt's directive). Shipped on its own with `scripts/dev-env/agent/model_stub_test.py`.
- **The confirmation covered the Send button.** "Queued." and "Sent" showed at the bottom right, on the composer's Send button at 1280, 1440 and phone widths, and a toast stays while the pointer is on it: after one click the next message could not be clicked through. They now show at the top centre; three sends back to back with the pointer still went through.
- **The page said "Completed" for up to 25 s into a queued message's turn.** A settled session is read every 25 s, so after a turn ended with messages waiting the header kept "Completed", with no Stop turn, until the slow read (observed 20 s). The queue's own read now reads the session again when a message leaves a settled session's queue; the header followed within 5 s on all three dispatches of the re-run.

**What it took, beyond m402's notes:** the first session in a new workspace failed with "sandbox create failed" (`namespaces "<ws>-sandbox" not found`); the namespace was there seconds later, and `agent-netpol <ws>` needs it anyway. The dashboard needs `VITE_AGENT_STREAM_URL=http://localhost:62100` or the transcript reads "not available" (the stream is refused by CORS on the API origin); `dev-env.sh` does not print it. `scripts/dev-env.test.sh` runs `clean` on `dev-10` and fails while `dev-10` is up. Another project's test run saturated the host once: the VM logged soft lockups for about 100 s, the gateway failed its probes and the forwards died; `dev-env.sh 10 up` healed it and the Stop was repeated. The VM's data disk stayed at 26 GiB free, above the 25 GiB guard.

**To repeat:** bring up as in m402's record, `agent-stub`, seed any placeholder at `tenants/default/agent-sessions/<ws>/model-key`, create a session with `{"agentConfig":{"agent":"claude","task":"… stub-hold:300"}}` and walk the steps with `X-Session-Token`.

## Gate: t011 (the live check) — cleared 2026-11-16, see "解阻记录 2026-11-16"

(The gate as it stood on 2026-11-14, kept for history.) **BLOCKED — needs, from the user or infra, a running stack with a sandbox cluster.** Everything below was verified against fakes and real Postgres only. `scripts/dev-env.sh <N> up` plus `agent-up` needs the shared kind/CAPD cluster, which the machine this ran on cannot bring up (no `kind` or `clusterctl`, an 8 GiB Docker VM, a ghcr pull that returned 504). The scenario: start an agent session on a long task; send two messages (both queue, in order, each shows its position); reorder them, edit one, remove the other; "send now" the remaining one and watch the running turn stop and the message run; send a third and let it run in order after the turn; press Stop turn on a running turn and confirm the session goes idle and steerable, the reply streamed so far stays in the transcript, the turn reads interrupted, and a follow-up message starts a fresh turn from the branch; then restart bex-api with messages queued and confirm they still run once. What only a live stack can show: that the sentinel written through the gateway exec path reaches the driver within the 500 ms poll, that the abort kills the engine's process tree for each profile (claude, codex, gemini, muse), the real latency to the "interrupted" state (about 15 s by reading the Completer), and what the delivery note says about unpushed work. Record the observed codes and timings under a "Live check" heading here, then run t010.

## 解阻记录 2026-11-14（/unblock w22）

- **来源：** 复核证据；本次没有新的用户决定，等用户选路径。
- **卡点：** t011 的实机检查要带 sandbox 集群的运行栈（dev-N 加 agent 腿），本机同样没有 `kind`、`clusterctl`，Docker VM 8 GiB，ghcr 504。
- **你能怎么帮（二选一，都不是必须）：**
  - **路径 1（默认）：** 同 m402：Docker Desktop 内存至少 16 GiB、ghcr.io 可拉取，之后 worker 自己装工具并跑 `scripts/mock-cluster.sh`。
  - **路径 2：** 周跑矩阵已经带 agent 腿（`BEX_VERIFY_AGENT_DRIVER=1`，w7/m393），可以把"运行中发消息得到 202、Stop 结束 turn"做成该腿上的探针。这超出了 D1 的授权范围，需要用户再点头。
- **关联：** w22/012 选了 A 方案后，同一次实机检查可以一并覆盖"被 Stop 的 turn 的改动在下一个 turn 里还在"。

## 解阻记录 2026-11-16（/unblock w22）

- **来源：** 用户决定 **D4**（2026-11-16）：对"本机搭建 + 生产探针"的推荐项回复 "ok, approve"。完整范围和复核证据记在 [m413 的同名记录](../m413/README.md)。本项用其中的**本机部分**；**没有批准**对生产做队列操作，生产探针只覆盖 m413 的"磁盘还在"。
- **本机部分（已批准）：**
  1. 装 `kind`、`clusterctl`（brew，或带 sha256 校验的二进制，worker 选）。
  2. colima 默认 profile 改成 8 CPU / 16 GiB 并启动（`bex-w21` 共用这个 Docker，启动前看一眼 `docker ps`）。
  3. 跑 `bash scripts/mock-cluster.sh`、`scripts/dev-env.sh <N> up` 和 `agent-up`。N 只能是 1 到 10，w22 没有自己的槽：先 `status` 选一个没起来的，用完 `down`；agent 腿是集群级独占。
  4. 模型只用 `agent-stub` 测试替身（固定回复，不花钱）；worker 要扩展它（`scripts/dev-env/agent/model-stub.py`），让 turn 能停留到被 Stop，这是代码，单独 `/ship`。**没有批准** `agent-openrouter`。
  5. 护栏（worker 的）：数据卷剩余磁盘低于 25 GiB 就停下，把读数写进新闸门。
  6. 结束：`dev-N down`；`docker ps` 没有别的容器时 `colima stop`。
- **对 t011 步骤 3 的修订（有依据）：** 替身只服务 Anthropic 端点（claude profile）。codex、gemini、muse 需要真实 key，而花钱的 key 没有批准，所以这几个引擎的"进程树被杀"记为 **not run**，写进 "Live check"，不假装验证过。要补，需要用户另批一次真实 key。其余步骤（排队、位置、重排、编辑、删除、send now、Stop 后续发、bex-api 重启后队列仍只执行一次）都在本地做。
- **闸门已清除：** 授权已给，机器经核实够用。bring-up 失败时，把精确原因写成新闸门再停靠。
- **worker 的下一步：** 与 m402、m413 共用同一次环境搭建；t011 在 m402 t006 之后做。
- **验收：** t011 的验收标准（在修订后的范围内）；然后 t010 收尾。
