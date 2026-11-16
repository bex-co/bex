# w6 · m434 — A Muse turn waits for its background work: the adapter holds the prompt open, and Muse workflows come back on

**Worker:** worker6 **Goal:** a Muse session that starts a workflow, a subagent or a backgrounded command keeps its turn open until that work, and the turn Muse starts to report it, have finished. The driver's turn then carries the result, and bex can offer Muse's Workflow tool again. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Adapter: an opt-in capability holds `session/prompt` until background work settles, and the host stays open meanwhile](t001.md) | 1h30m | — |
| t002 | [Adapter: the turn Muse starts to report finished work streams into the held prompt, with its approvals and cancellation](t002.md) | 1h30m | t001 |
| t003 | [Adapter: a real-host loopback test on Muse 1.4.14 with a workflow and a backgrounded command](t003.md) | 1h | t002 |
| t004 | [Release the adapter through its publish workflow](t004.md) | 30m | t003 |
| t005 | [bex pins the new adapter and negotiates the capability; a driver test holds the prompt across an in-progress workflow](t005.md) | 1h | t004 |
| t006 | [Muse's Workflow tool is back on (`auto`), and ADR108 D8 is amended](t006.md) | 30m | t005 |
| t007 | [Production: a Muse routine run that calls `workflow` opens its PR or reports that it has nothing to change](t007.md) | 45m | t006 |
| t008 | [Devin parity](t008.md) | 30m | t006 |
| t009 | [Simplify](t009.md) | 20m | t008 |
| t010 | [Test coverage](t010.md) | 45m | t008 |
| t011 | [Closeout](t011.md) | 10m | t007, t010 |

## Definition of done

- Under a capability the client negotiates, `@bex-co/muse-code-acp` answers `session/prompt` only after three conditions hold: no workflow, subagent or background tool call is in progress; no host-owned turn is running; and no approval or user input is pending. The host-owned turn's updates stream into the same prompt. The host is never closed while such work runs. A client that does not negotiate the capability sees 0.11.0's behavior. Fixture tests prove it, and so does a loopback test on Muse 1.4.14-R7309.1 in the adapter's required CI.
- An adapter release carrying it is on npm, published by the adapter's `publish.yml`. bex pins that release in `lego/backend/internal/agentsession/agent-profiles.json`.
- The driver negotiates the capability. A driver test keeps the agent's prompt open while a workflow tool call is `inProgress`, and records the workflow's result before the turn ends.
- `writeMuseSettings` writes `run.workflow_trigger_mode: "auto"`. ADR108 D8 carries a dated amendment, and the agent-image README matches it.
- After the production deploy that carries the change, a Muse `rt-*` routine run that calls `workflow` opens its PR or reports that it has nothing to change. Its transcript shows the workflow's result before `finish`.

## Source + Goal linkage

- **Source:** promoted from [w6/130](../done/130.md) on 2026-11-16. That note was split from [w6/129](../blocked/129.md), which turned Muse's Workflow tool off as a stopgap (`5f29b4e81`, [ADR108 D8](../../../docs/ADR108-muse-code-agent.md)). The production evidence is in w6/m354 t007 and ADR105's routine table: three of 19 Muse sessions called `workflow` and ended their turn 2 to 3 s later with their work undone.
- **Goal linkage:** cloud coding agents with Devin as the reference ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md)), for the Muse profile ([ADR108](../../../docs/ADR108-muse-code-agent.md)). A Devin session keeps working until its work is done.
- **Expected outcome:** Muse can fan work out in parallel inside a bex turn. A long build or test that Muse 1.4.14's `bash` sends to the background (past `yield_time_ms`, at most 300000) also no longer ends a turn early. That second failure has not been seen on production, but the 0.11.0 adapter does not guard against it.
- **Why now:** w6/129 left Muse on bex without parallel fan-out. The adapter's 0.11.0 behavior is the only thing in the way, and every piece of it is in code that bex-co maintains.
- **Two repositories.** t001–t004 run in [`bex-co/muse-code-acp`](https://github.com/bex-co/muse-code-acp), under its own `AGENTS.md`, `.pm/DO_NOT_DO.md`, `ship` skill (which pushes to its `main`) and `release` skill. The release skill says: "A request to release authorizes the release commit, push, tag, workflow dispatch, and GitHub release. A request only to edit this skill or ship code does not authorize publication." The worker first confirms that its run authorizes pushing to the adapter's `main` and publishing a release (the user's request, or the adapter's own `/loopx`). If it does not, the milestone parks with that authorization as its gate, naming whatever is already done.
- **Devin parity, not Render parity:** the change is visible on the agent session surface (transcript and phase), and for that surface Devin is the reference product.
