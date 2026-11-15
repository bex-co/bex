# w22 · m413 — A Stop turn keeps its work: the interrupted turn's disk is retained and the next message continues from it, as on Devin

**Worker:** worker22 **Goal:** after a user stops a turn, the agent's uncommitted and unpushed work is still in the checkout when the next message starts, for a bounded time and with the user told how long, and Stop shows within seconds. **Status:** todo (t001–t003 done and shipped)

## Tasks (in order)

| id | title | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------- | ----- | ------------------ |
| t001 | Stop shows within seconds: the Completer reads a stop-requested session's status on a short tick — **DONE** | 45m | — |
| t002 | Retain: an interrupted turn's disk is kept as an image, counted by the storage cap, bounded in time and deleted when it ends — **DONE** | 3h | t001 |
| t003 | Continue: the next message boots a fresh sandbox from the retained image, and falls back to a fresh one when it is gone — **DONE** | 2h30m | t002 |
| t004 | Show retention: the dashboard, API and CLI say how long the changes are kept, and that they were lost if they expired | 1h30m | t002 |
| t005 | Render parity (Devin) | 30m | t003, t004 |
| t006 | Simplify | 20m | t005 |
| t007 | Test coverage | 45m | t005 |
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
