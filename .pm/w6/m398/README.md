# w6 · m398 — A turn's transcript survives a sandbox that dies mid-turn

**Worker:** worker6 **Goal:** a session whose sandbox dies mid-turn keeps its conversation and commands up to about 30 seconds before the death, on every surface, so the failure can be investigated. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Driver: flush a running turn's completed transcript parts to bex-api | 2h | — |
| [t002](t002.md) | Completer: a failed turn keeps its flushed parts and says why the rest is missing | 1h | t001 |
| [t003](t003.md) | REST, GraphQL, MCP and the session page show a failed turn's saved transcript | 45m | t002 |
| [t004](t004.md) | Production check: a QA session whose sandbox is killed mid-turn keeps its transcript | 45m | t003 |
| [t005](t005.md) | Render parity (Devin): a failed session's history reads alike everywhere | 20m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 45m | t005, t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

## Definition of done

On production, a session whose sandbox Pod is killed mid-turn shows its transcript up to about 30 s before the kill on REST, GraphQL, MCP and the session page, marked truncated with the reason. A later harvest of a turn never duplicates what was flushed.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w6` (2026-11-14), item 2. Found while investigating the 2026-11-13 `rt-flatten-abstractions` OOM (session `ags-dbrhfr0k58is73auo6a0`, 0 transcript rows after 371 model requests). Production: 9 failed sessions in 30 days with model spend kept no transcript.
- **Goal linkage:** ADR008 pillar 5 (hosted agent sessions). ADR047 consequence 5 promises a durable transcript store for terminal sessions; ADR051's harvest cannot run once the sandbox is gone.
- **Expected outcome:** a crashed or OOM-killed turn can be read and debugged.
- **Why now:** the routines run every three hours on 8 GiB sandboxes, and the next OOM's cause (w6/127 suspects `/tmp`) cannot be confirmed without its transcript.
- **Render parity:** included (Devin is the reference for agent sessions): the transcript surfaces change.
