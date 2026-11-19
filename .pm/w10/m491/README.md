# w10 · m491 — An agent session's unpushed work survives into the next message however its turn ended, as on Devin

**Worker:** worker10 **Goal:** a turn that ends with uncommitted changes or unpushed commits, whether it completed, failed or sat idle until its sandbox was closed, leaves that work in the checkout of the session's next turn for a bounded time, and every turn whose work was kept or lost says so to the user and to the next turn's agent. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Design: every turn end with undelivered work keeps its disk; ADR047 D12 and ADR018 widened | 30m | — |
| [t002](t002.md) | Facts: a failed turn reports its checkout's Git facts too, and the turn records them | 1h15m | t001 |
| [t003](t003.md) | A failed turn with undelivered work keeps its disk instead of being torn down at once | 1h30m | t002 |
| [t004](t004.md) | The next message keeps a completed turn's undelivered work: save before the previous sandbox is replaced | 2h | t002 |
| [t005](t005.md) | The idle close keeps a completed turn's undelivered work: save instead of terminate | 1h15m | t004 |
| [t006](t006.md) | Say what was kept or lost: REST, GraphQL, MCP, and the next turn's agent | 1h30m | t003, t005 |
| [t007](t007.md) | Dashboard and `bex agent` show it on every turn that kept or lost work | 1h15m | t006 |
| [t008](t008.md) | Render parity (Devin) | 30m | t007 |
| [t009](t009.md) | Simplify | 20m | t008 |
| [t010](t010.md) | Test coverage | 45m | t008, t009 |
| [t011](t011.md) | Live check on `dev-10`: a failed, a followed-up and an idle-closed turn keep their work | 1h30m | t010 |
| [t012](t012.md) | Closeout | 10m | t011 |

Total: **12 tasks, about 12h30m**.

## Definition of done

1. A turn that **completes** with an uncommitted file and an unpushed commit, followed by a message within the idle grace, runs that message in a sandbox whose checkout still has both. A real-Postgres test with the sandbox fake fails on today's code, where `runSteerDispatch` tears the previous sandbox down and boots a fresh clone.
2. The same turn left alone past the idle grace (`BEX_AGENT_SANDBOX_IDLE_TTL`) keeps its disk when the reaper closes the sandbox, and a later message continues from it while it is kept. A clean checkout is closed exactly as today, with nothing saved.
3. A turn that **fails** with undelivered work (a driver error, the no-progress watchdog, the turn ceiling, a missing structured output) keeps its disk the same way instead of being torn down at once. A sandbox already gone (`failLostSandbox`) records the loss and the reason.
4. Every turn whose undelivered work was kept, continued from or lost says so on REST, GraphQL, MCP, the dashboard and `bex agent`, with the time it is kept until or the reason it was lost, and what was at stake (the number of unpushed commits and whether there were uncommitted changes). The next turn's agent is told in its prompt whether it starts from the restored checkout or from a fresh one that lost that work.
5. m413's rules hold for every kept disk unchanged: counted against the workspace storage cap, kept for `BEX_AGENT_INTERRUPT_RETENTION` (or its renamed successor), deleted on expiry, cancel, delete, archive, workspace delete and after a continuation; an open editor, a full cap or a failed save degrades to today's fresh start with the loss shown.
6. ADR090 is unchanged: neither the driver nor bex-api commits or pushes anything. ADR047 D12 and ADR018 record the widened design and its Devin basis.
7. Live on `dev-10` with the `agent-stub` test double and real sandboxes (m413's recipe): the three cases in items 1–3 each show the uncommitted file and the unpushed commit in the next turn's sandbox, and a shortened retention shows the loss. The production `isolation-matrix.yml -f verify_agent_retention=true` probe still passes.

## Source + Goal linkage

- **Source:** inbox note [w10/013](../done/013.md), filed 2026-11-18 from the agent-automation runs of 2026-11-15..18 (sessions `ags-dbu0pfdikg6s738lam7g`, `ags-dbu0pfmc1l2s739d3n3g`, `ags-dbutgej906ns73ff3kb0`), promoted by `/loopx w10` on 2026-11-18. It also covers the last clause of [w10/016](../016.md) ("before failing a turn on the watchdog, snapshot unpushed work"); 016's own fix (counting a running tool's output as progress) stays there.
- **Triage evidence (main at `fd9787d29`, 2026-11-18):**
  - Retention is gated on a Stop: `settleCompleted` keeps a disk only when `outcome == turnOutcomeInterrupted` (`lego/backend/internal/agentsessions/completion.go:756`), and `turnRetention` answers `none` for any turn that is not `Interrupted` (`turn_retention.go:85`).
  - The loss is wider than the note says. Every follow-up message tears the previous sandbox down and boots a fresh one (`runSteerDispatch`, `service.go:2166-2190`; ADR047 D8, a new prompt cannot ride the old sandbox), so a completed turn's unpushed work is gone at the **next message**, not only after the idle close. `dispatchImage` boots the retained image, else the repository environment's image, else the template: none holds the old checkout.
  - A failed turn skips the idle grace and is torn down at its settle (`teardown`, `skipGrace := … || record.Phase == PhaseFailed`, `completion.go:997`).
  - A completed turn's idle close calls `terminate` with no save (`completion.go:1009`).
  - The driver already measures what is at stake on a successful or stopped turn (`GitFacts.dirty`, `unpushedCommits`, `lego/agent-image/driver/src/delivery-check.ts`), but its failed status carries no `git` object (`session.ts:513-517`, and the setup failure at `:124`), so a failed turn's stake is unknown today.
- **Reference product (Devin):** a session's machine is kept while it sleeps and resumes with its state for 14 days ([Devin release notes](https://docs.devin.ai/release-notes/2026), 2026-09-25, 2026-10-05, 2026-10-09), whatever ended the last exchange; a follow-up never discards the machine. w22/m413 cites the same basis.
- **Design direction (t001 settles it):** reuse m413's retained-image mechanism (`retention.go`, `completion_retention.go`, `agent_session_retained_images`) and widen when it runs: at a failed turn's settle, at a follow-up before the previous sandbox is torn down, and at the idle close. Save only when the checkout has undelivered work (a clean tree skips the pause, closing one of m413's follow-ups for these paths). A push to a recovery ref, the note's other option, would make bex-api push work the agent did not deliver and conflicts with ADR090's custody, so it is not taken.
- **Goal linkage:** ADR008 pillar 5 (cloud coding agents, [ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md)); unattended automations are the product's main use, and a run that loses its work delivers nothing the user paid for.
- **Expected outcome:** an agent whose push was refused ([w10/014](../014.md)), whose turn failed or which was simply followed up finds its commits and edits where it left them, and a user whose work could not be kept is told so instead of finding a clean clone.
- **Why now:** three production sessions lost work in two days, one a verified fix; the w10/014 push refusal makes it recur on every session pinned to an older commit.
- **Render parity included (reference product: Devin):** the turn's retention fields change on REST, GraphQL and MCP, and the dashboard and `bex agent` show them.
