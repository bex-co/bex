# w6 · m563 — An agent session's sandbox closes 5 minutes after its turn, as Devin's session sleeps; an automation's closes at once

**Worker:** worker6 **Goal:** a finished turn's sandbox stops costing money within minutes, as a Devin session sleeps after 5 minutes of inactivity, instead of running for two more hours that nothing uses. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [A person's session closes 5 minutes after its turn, and an automation's or a child's at its harvest](done/t001.md) — **DONE** | 1h | — |
| t002 | [An Enterprise workspace sets its own idle timeout, 5 to 120 minutes, as Devin's Enterprise customers can](done/t002.md) — **DONE** | 2h | t001 |
| t003 | [ADR059, ADR047, `lego/backend/AGENTS.md` and the dashboard say 5 minutes](done/t003.md) — **DONE** | 30m | t001, t002 |
| t004 | [Production check: idle time per sandbox falls to the new bounds, and an idle close keeps undelivered work](done/t004.md) — **DONE** | 1h | t003 |
| t005 | [Render parity (Devin for agent sessions)](done/t005.md) — **DONE** | 30m | t004 |
| t006 | [Simplify](done/t006.md) — **DONE** | 20m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 45m | t005 |
| t008 | [Closeout](done/t008.md) — **DONE** | 10m | t006, t007 |

## Definition of done

- A session a person started (no `automation_id`, no `parent_session_id`) has its sandbox terminated 5 minutes after its turn ends or its last editor SSH disconnects, whichever is later. An open editor SSH session still keeps it, as today.
- An automation's session and a child session have their sandbox terminated at the Completer's harvest, with no idle grace. An open editor SSH session still keeps it.
- A failed or archived session behaves as today (no grace).
- A turn whose checkout holds undelivered work still keeps its disk at that close (w10/m491's `retainOnClose`), so closing earlier loses nothing.
- An Enterprise workspace can set the timeout for its sessions to 5–120 minutes on REST, GraphQL, MCP and the dashboard; any other plan is refused with the plan error and keeps 5 minutes.
- On production, over 24 h after the deploy, the median billed time a person's session sandbox runs past its last turn is at most 6 minutes, and an automation's at most 1 minute (read from `usage_hourly`, `sandbox_meter_states` and `agent_session_turns`, as in § Source).
- ADR059, ADR047, `lego/backend/AGENTS.md` and the dashboard no longer say 2 hours.

## Source + Goal linkage

- **Source:** the user's decisions of 2026-11-22: "那就跟 devin 对齐" (align the idle close with Devin: 5 minutes by default, 5–120 minutes for Enterprise) and "其他的按照你推荐的修" (automation sessions close at once, as recommended). This replaces the user's own 2026-10-25 choice of two hours ([ADR059 § Superseded](../../../../docs/ADR059-agent-sandbox-hibernation.md#2026-10-25--superseded-hibernation-retired)), which was made when an idle close lost undelivered work. w10/m491's retention (production carries `0d7e8fc8b`) now keeps that work.
- **Reference product:** Devin. Per [docs.devin.ai/admin/billing/usage](https://docs.devin.ai/admin/billing/usage) (read 2026-11-22), a session "sleeps automatically after 5 minutes of inactivity by default"; Windows and GPU machines sleep after 30 minutes; Enterprise customers can ask for 5–120 minutes, which applies to every session type; a sleeping session consumes no usage and wakes on the next message. bex has no Windows or GPU sandbox, so that row does not apply. Closing an automation's sandbox at once is stricter than Devin's 5 minutes for unattended sessions. It is the recommendation the user approved, not a parity claim: no person opens an editor on a routine's sandbox.
- **Evidence (production, read-only, 2026-11-22):** in the `bex` workspace (`tea-d98210cbbpdc73dcrkvg`), November's 301 agent sandboxes billed $230.34 for 578 hours. Their turns ran 122 of those hours. $185.94 (81%) was the idle grace, and the median idle tail was exactly 120 minutes. 258 of the 301 belonged to automations. None of the 301 served a second turn, because a follow-up always boots a fresh sandbox (`Steer`, `lego/backend/internal/agentsessions/service.go:2130`). After a turn ended, people opened Zed 25 times with a known delay: 20 within 5 minutes, 2 more within 15.
- **Goal linkage:** ADR047 cloud coding-agent sessions, priced and behaving as the reference product does ([ADR008](../../../../docs/ADR008-vision.md)).
- **Expected outcome:** at November's volume the `bex` workspace's agent sandbox charge falls from about $231 to about $45, and a customer's sessions stop paying for two idle hours per turn.
- **Why now:** the billing page's projection for the period is $892, mostly this idle tail. Every customer session pays it. The retention that made the long grace unnecessary shipped on 2026-11-18.
- **Render parity:** included, against Devin: t002 adds a setting on REST, GraphQL, MCP and the dashboard.
- **Related:** m564 (opening a closed session in Zed starts its sandbox), which makes the short grace painless for editors. w10/m491 t011 is the live check of the retention this relies on.

## Parked (2026-11-23)

`b5cbb23493` ships t001–t003 and t005–t007: a person's session closes 5 minutes after its turn, an automation's and a child's at the harvest, and an Enterprise workspace sets `idleTimeoutMinutes` (5–120) on REST, GraphQL, MCP, Settings → Agents and `bex agent env set --idle-timeout`. Production sets no `BEX_AGENT_SANDBOX_IDLE_TTL` (`lego/operator/config` has none), so the new 5-minute default reaches it with the pin. t004 needs that deploy and 24 hours of sessions after it: the gate is the pin carrying `b5cbb23493` and a date gate a day past it. When it clears, read the deploy time from the pin, and if 24 hours have not passed since it, park again with a date gate 24 hours after it; then run t004's read on sandboxes created after the deploy (person and automation medians, the idle share) and its close-and-continue check as the QA user (`bash scripts/qa-login.sh`), record the figures here, and close with t008.

## Unblocked (2026-11-24)

The gate cleared by itself (verified evidence, no user decision). The newest pin on `origin/main`, `d9dd27d687` (`chore(deploy): pin platform images to ff5de0a130e6`, 2026-11-24 06:28Z), carries `b5cbb23493` (`git merge-base --is-ancestor b5cbb23493 ff5de0a130e6`), and `bex-api` and `bex-ssh-gateway` ran that pin's `bex-operator@sha256:93c96486b2b1…`. The first pin to carry it was `e6c65f5c62` (`… to 1a5a8e872398`, 2026-11-23 04:25:41Z). Its `deploy (bex via Argo)` run rolled bex onto it by 04:31:28Z. 24 hours after that was 2026-11-24 04:31Z, before this resume at 12:00Z, so the item was not parked again. Next: t004, then t008.

## Production check (2026-11-24, t004)

The read covers the agent sandboxes bound from 2026-11-23 04:31:28Z, the roll above, to 2026-11-24 12:25Z. Production ran five pins in that time. Two of the later ones changed what holds a sandbox open, so each sandbox is read against the pin it ran under (rolled = its deploy run or the `bex-api` ReplicaSets):

| pin | rolled | what it changed for this check |
| --- | --- | --- |
| `1a5a8e872398` | 11-23 04:31Z | `b5cbb23493`: a person's sandbox closes 5 minutes after the turn, an automation's at the harvest |
| `ac882b5f557f` | 11-23 11:06Z | nothing; the session page's conversation stream still counted as an editor |
| `9fdf7e0a1171` | 11-23 17:38Z | `2d68de52fc` (m564 t009): only SSH logins keep a sandbox or move its idle clock |
| `f46c44d9ddb1` | 11-24 00:01Z | nothing |
| `ff5de0a130e6` | 11-24 06:31Z | `40a947b2db` (w6/171): the page ends the stream each re-attach replaces |

**The read.** It ran read-only on the replica `bex-db-1`, with the kubeconfig from `scripts/fetch-app-kubeconfig.sh` in a temporary directory deleted afterwards. A sandbox's billed seconds are `SUM(usage_hourly.quantity)` (`resource_kind = 'sandbox'`, `kind = 'sandbox_compute_seconds'`) divided by its `sandbox_meter_states.weight_milli`. Its idle tail is the end of its last `sandbox_running_intervals` row minus its turn's `agent_session_turns.completed_at`, joined through `agent_session_sandboxes` (`turn`). For every sandbox below, the billed seconds equal its running interval to the second.

**An automation's median tail is 0.9 s (the bound is 1 minute).**

- In the 24 hours after the deploy (11-23 04:31Z to 11-24 04:31Z), `bex`'s routines ran 14 automation sandboxes. Their median tail was 0.86 s and the longest 1.8 s. They billed 3.41 hours ($1.48), their turns ran 3.41 hours of it, and the idle tail cost $0.0016 (0.1%). Before the deploy (November 1–22) the median tail was 120 minutes and the idle grace was 81% of the charge.
- Through 11-24 12:00Z, 18 automation sandboxes closed, and each tail was at most 2 s under every pin (1, 4, 5, 4 and 4 sandboxes). None was held open by a session page under the two pins before `2d68de52fc`.

**A person's median tail is 5 min 6 s (the bound is 6 minutes).** Only bex's own workspaces ran agent sessions after the deploy: 33 sessions in `agent_model_usage_events`, all in `bex` and `tian-personal`. No person outside QA started one. Each session a person started was a disposable QA session, deleted afterwards, so its `agent_session_turns` rows are gone, while the meter's rows stay. In `bex`, the turn end each run recorded on the board gives six turn sandboxes:

| pin | sandbox | run | turn ended | billed until | tail |
| --- | --- | --- | --- | --- | --- |
| `ac882b5f557f` | `9cfd4ddb…` | m564 t004 | 11-23 11:56:27Z | 12:05:15Z | 8 min 48 s: a page view at 12:00:12Z moved the idle clock, which `2d68de52fc` fixed |
| `9fdf7e0a1171` | `d6599e3a…` | m564 t004's last check | 11-23 17:52:25Z | 17:57:35Z | 5 min 10 s |
| `f46c44d9ddb1` | `76fd2fd3…` | w6/168 | 11-24 02:04:44Z | 02:09:45Z | 5 min 1 s |
| `ff5de0a130e6` | `f6d5c07f…` | w6/171 | 11-24 09:33:44Z | 09:38:44Z | 5 min 0 s |
| `ff5de0a130e6` | `365bdc66…` | this check, turn 1 (clean tree) | 11-24 12:07:44Z | 12:12:58Z | 5 min 13 s |
| `ff5de0a130e6` | `dcaefc91…` | this check, turn 2 (dirty, disk kept) | 11-24 12:13:44Z | 12:18:45Z | 5 min 1 s |

- The median of the six is 5 min 6 s; under the pins carrying `2d68de52fc` (the last five) it is 5 min 1 s. Each paid about 5 minutes of idle compute at weight 2422 (about $0.018) instead of 120 minutes (about $0.43). Idle is 91% of these six sandboxes' 37 billed minutes because a QA turn runs for under a minute.
- The sandboxes m564 and w6/171 started for an editor closed 5 min 4 s (`52f45820…`, `9fdf7e0a1171`) and 5 min 12 s (`1d3b3f88…`, `ff5de0a130e6`) after the last SSH disconnect. `58d8b16c…` (`ac882b5f557f`) closed 5 min 16 s after it, about 5 minutes after the page's stream ended at 12:07:36Z. `7ea0f7c8…` (`ac882b5f557f`) was held by the open page until 5 minutes after the tab left, the case `2d68de52fc` fixed. The other two (`c622fb71…`, `ed8e97fe…`) ended within 30 s of their last use, when their runs deleted the sessions.
- In `tian-personal`, 23 agent sandboxes and 2 environment builds belong to the disposable QA runs of w4/m544, w10/m561, w10/m577, w10/m597, w10/m599, w10/038 and w10/052, all deleted. With no turn rows left they are not counted above. None of the 25 ran longer than 6 min 11 s in all, turn included, and the three that outlived their session's last model call ended 5 min 9 s to 5 min 14 s after it.

**An idle close keeps undelivered work into the next message.** As the QA user (`bash scripts/qa-login.sh`, logged out afterwards) in `bex`, which reads `idleTimeoutMinutes: 5` and `idleTimeoutConfigurable: false` (Scale): session `ags-dc2nro4b6mms7391klm0` (disposable, deleted afterwards) on the private fixture `bex-co/bex-hello-go-live`, `claude` on `anthropic/claude-haiku-5.5`.

- **Turn 1** was told to leave a file uncommitted. After ADR090's post-turn delivery prompt the agent committed it anyway, pushed and opened fixture PR #14. With a clean tree, the close ended the sandbox 5 min 13 s after the turn and kept nothing.
- **Turn 2** was a message, so it ran in a fresh sandbox (`dcaefc91…`). It wrote `w6-m563-uncommitted.txt`, declined the delivery prompt, and ended at 12:13:44Z with `dirty=true unpushed=0`. At 12:18:44Z, 5 minutes later, `bex-api` logged `agent-session retention: keeping the disk (… turn=2 cause=idle_closed dirty=true unpushed=0)`. The meter stopped at 12:18:45Z, and at 12:20:54Z it logged `kept … bytes=2381149513 until=2026-11-25T12:20:54Z`.
- **Turn 3** was sent at 12:23:30Z and booted `e132baf6…` from the kept disk. `cat w6-m563-uncommitted.txt; git status --short` printed `w6-m563 probe kept across the 5-minute idle close` and `?? w6-m563-uncommitted.txt`. Turn 2's row now reads `retention = continued`, `retention_cause = idle_closed`.
- **Cleanup.** The session was deleted at 12:24:16Z, which ended its sandbox and left no retained image. PR #14 was closed and `bex-agent/ags-dc2nro4b6mms7391klm0` deleted.

**Reproduce.** Run the read above on `bex-db-1` with `psql -d bex` (`kubectl -n bex-system exec -i bex-db-1 -c postgres`). The close and retention lines come from `kubectl -n bex-system logs` of both `bex-api` pods (`deferring teardown, …`, `agent-session retention: …`). The session state comes from `GET https://api.bex.co/v1/agent-sessions/<id>` and its `/transcript`, with the cookie jar `qa-login.sh` prints. A deleted QA session's turn end is the time its board record gives, and its sandbox is the `sandbox_running_intervals` row of that time in its workspace.

**Against the definition of done.** `b5cbb23493` shipped the code (t001–t003, t005–t007). On production, a person's median idle tail is 5 min 6 s (5 min 1 s since `2d68de52fc`) and an automation's is 0.9 s. An idle-closed turn with an uncommitted file kept its disk, and the next message's checkout read the file. Closed 2026-11-24.
