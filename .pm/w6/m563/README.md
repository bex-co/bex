# w6 · m563 — An agent session's sandbox closes 5 minutes after its turn, as Devin's session sleeps; an automation's closes at once

**Worker:** worker6 **Goal:** a finished turn's sandbox stops costing money within minutes, as a Devin session sleeps after 5 minutes of inactivity, instead of running for two more hours that nothing uses. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [A person's session closes 5 minutes after its turn, and an automation's or a child's at its harvest](t001.md) | 1h | — |
| t002 | [An Enterprise workspace sets its own idle timeout, 5 to 120 minutes, as Devin's Enterprise customers can](t002.md) | 2h | t001 |
| t003 | [ADR059, ADR047, `lego/backend/AGENTS.md` and the dashboard say 5 minutes](t003.md) | 30m | t001, t002 |
| t004 | [Production check: idle time per sandbox falls to the new bounds, and an idle close keeps undelivered work](t004.md) | 1h | t003 |
| t005 | [Render parity (Devin for agent sessions)](t005.md) | 30m | t004 |
| t006 | [Simplify](t006.md) | 20m | t005 |
| t007 | [Test coverage](t007.md) | 45m | t005 |
| t008 | [Closeout](t008.md) | 10m | t006, t007 |

## Definition of done

- A session a person started (no `automation_id`, no `parent_session_id`) has its sandbox terminated 5 minutes after its turn ends or its last editor SSH disconnects, whichever is later. An open editor SSH session still keeps it, as today.
- An automation's session and a child session have their sandbox terminated at the Completer's harvest, with no idle grace. An open editor SSH session still keeps it.
- A failed or archived session behaves as today (no grace).
- A turn whose checkout holds undelivered work still keeps its disk at that close (w10/m491's `retainOnClose`), so closing earlier loses nothing.
- An Enterprise workspace can set the timeout for its sessions to 5–120 minutes on REST, GraphQL, MCP and the dashboard; any other plan is refused with the plan error and keeps 5 minutes.
- On production, over 24 h after the deploy, the median billed time a person's session sandbox runs past its last turn is at most 6 minutes, and an automation's at most 1 minute (read from `usage_hourly`, `sandbox_meter_states` and `agent_session_turns`, as in § Source).
- ADR059, ADR047, `lego/backend/AGENTS.md` and the dashboard no longer say 2 hours.

## Source + Goal linkage

- **Source:** the user's decisions of 2026-11-22: "那就跟 devin 对齐" (align the idle close with Devin: 5 minutes by default, 5–120 minutes for Enterprise) and "其他的按照你推荐的修" (automation sessions close at once, as recommended). This replaces the user's own 2026-10-25 choice of two hours ([ADR059 § Superseded](../../../docs/ADR059-agent-sandbox-hibernation.md#2026-10-25--superseded-hibernation-retired)), which was made when an idle close lost undelivered work. w10/m491's retention (production carries `0d7e8fc8b`) now keeps that work.
- **Reference product:** Devin. Per [docs.devin.ai/admin/billing/usage](https://docs.devin.ai/admin/billing/usage) (read 2026-11-22), a session "sleeps automatically after 5 minutes of inactivity by default"; Windows and GPU machines sleep after 30 minutes; Enterprise customers can ask for 5–120 minutes, which applies to every session type; a sleeping session consumes no usage and wakes on the next message. bex has no Windows or GPU sandbox, so that row does not apply. Closing an automation's sandbox at once is stricter than Devin's 5 minutes for unattended sessions. It is the recommendation the user approved, not a parity claim: no person opens an editor on a routine's sandbox.
- **Evidence (production, read-only, 2026-11-22):** in the `bex` workspace (`tea-d98210cbbpdc73dcrkvg`), November's 301 agent sandboxes billed $230.34 for 578 hours. Their turns ran 122 of those hours. $185.94 (81%) was the idle grace, and the median idle tail was exactly 120 minutes. 258 of the 301 belonged to automations. None of the 301 served a second turn, because a follow-up always boots a fresh sandbox (`Steer`, `lego/backend/internal/agentsessions/service.go:2130`). After a turn ended, people opened Zed 25 times with a known delay: 20 within 5 minutes, 2 more within 15.
- **Goal linkage:** ADR047 cloud coding-agent sessions, priced and behaving as the reference product does ([ADR008](../../../docs/ADR008-vision.md)).
- **Expected outcome:** at November's volume the `bex` workspace's agent sandbox charge falls from about $231 to about $45, and a customer's sessions stop paying for two idle hours per turn.
- **Why now:** the billing page's projection for the period is $892, mostly this idle tail. Every customer session pays it. The retention that made the long grace unnecessary shipped on 2026-11-18.
- **Render parity:** included, against Devin: t002 adds a setting on REST, GraphQL, MCP and the dashboard.
- **Related:** m564 (opening a closed session in Zed starts its sandbox), which makes the short grace painless for editors. w10/m491 t011 is the live check of the retention this relies on.
